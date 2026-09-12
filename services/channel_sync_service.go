package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go-saas/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// DTO Structs parse Response từ Mock Shopee API
type ShopeeItemListResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Response  struct {
		Item []struct {
			ItemID     int64   `json:"item_id"`
			ItemStatus string  `json:"item_status"`
			ItemSKU    string  `json:"item_sku"`
			ItemName   string  `json:"item_name,omitempty"` // Tùy chọn nếu mock API trả thêm tên
			Price      float64 `json:"price,omitempty"`
			Quantity   int     `json:"quantity,omitempty"`
		} `json:"item"`
		TotalCount  int  `json:"total_count"`
		HasNextPage bool `json:"has_next_page"`
		NextOffset  int  `json:"next_offset"`
	} `json:"response"`
}

// Service chính nhận *gin.Context làm tham số truyền vào
func SyncProductsFromChannel(c *gin.Context, storeID uint64, channelType string) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("không thể lấy DB từ context: %w", err)
	}

	userID := c.GetUint64("user_id")

	// 1. Gọi Mock API Shopee bằng utils.RequestMock
	mockURL := fmt.Sprintf("%s/api/v2/shopee/product/get_item_list?offset=%s&page_size=%s&item_status=%s",
		getShopeeMockBase(c), c.DefaultQuery("offset", "0"), c.DefaultQuery("page_size", "10"), c.DefaultQuery("item_status", "NORMAL"))

	respBytes, err := utils.RequestMock(c, "GET", mockURL, map[string]string{
		"Authorization": c.GetHeader("Authorization"),
	}, nil)

	var shopeeData ShopeeItemListResponse

	// Nếu có lỗi từ RequestMock nhưng respBytes vẫn có dữ liệu (Ví dụ Mock Server trả lỗi 400/500 kèm body JSON)
	if err != nil {
		if len(respBytes) > 0 {
			if parseErr := json.Unmarshal(respBytes, &shopeeData); parseErr == nil && shopeeData.Message != "" {
				return fmt.Errorf("lỗi từ Mock API Shopee: %s (error: %s)", shopeeData.Message, shopeeData.Error)
			}
		}
		return fmt.Errorf("lỗi gọi mock API shopee: %w", err)
	}

	// Unmarshal dữ liệu JSON nhận được khi thành công
	if err := json.Unmarshal(respBytes, &shopeeData); err != nil {
		return fmt.Errorf("lỗi parse json mock shopee: %w", err)
	}

	// 2. Đọc setting tự động sync
	autoCreateSetting := utils.GetSetting(c, "auto_create_product_on_sync", "false")
	autoCreate := autoCreateSetting == "true"

	// 3. Khởi tạo Transaction với sqlx
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Lặp qua danh sách Item kéo về từ Sàn
	for _, item := range shopeeData.Response.Item {
		itemIDStr := fmt.Sprintf("%d", item.ItemID)
		skuCode := item.ItemSKU
		if skuCode == "" {
			skuCode = fmt.Sprintf("SKU-SP-%d", item.ItemID)
		}

		err := processSingleShopeeItem(c, tx, userID, storeID, channelType, itemIDStr, skuCode, item.ItemName, item.Price, item.Quantity, autoCreate)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Core Logic xử lý từng Item trong Transaction
func processSingleShopeeItem(c *gin.Context, tx *sqlx.Tx, userID, storeID uint64, channelType, channelProductID, skuCode, productName string, price float64, quantity int, autoCreate bool) error {
	var mappedID uint64

	// BƯỚC 1: Kiểm tra xem bản ghi đã có trong bảng mapping chưa
	checkQuery := `SELECT id FROM channel_product_mappings WHERE user_id = ? AND channel_type = ? AND channel_sku_id = ?`
	utils.LogSQL(checkQuery, userID, channelType, channelProductID)
	err := tx.Get(&mappedID, checkQuery, userID, channelType, channelProductID)

	if err == nil {
		// Đã tồn tại -> Cập nhật thông tin mới từ sàn
		updateQuery := `UPDATE channel_product_mappings SET channel_price = ?, channel_quantity = ?, channel_sku_code = ?, updated_at = NOW() WHERE id = ?`
		utils.LogSQL(updateQuery, price, quantity, skuCode, mappedID)
		_, err = tx.Exec(updateQuery, price, quantity, skuCode, mappedID)
		return err
	}

	// BƯỚC 2: Chưa map -> Kiểm tra xem mã SKU có trùng trên Web không (Auto-Match)
	type VariantMatch struct {
		ID        uint64 `db:"id"`
		ProductID uint64 `db:"product_id"`
	}
	var match VariantMatch

	matchQuery := `SELECT id, product_id FROM product_variants WHERE sku = ? LIMIT 1`
	utils.LogSQL(matchQuery, skuCode)
	errMatch := tx.Get(&match, matchQuery, skuCode)

	if errMatch == nil {
		// Trùng SKU -> Auto Match (status = MAPPED)
		insertMapQuery := `
			INSERT INTO channel_product_mappings 
				(user_id, store_id, channel_type, product_id, variant_id, channel_product_id, channel_sku_id, channel_sku_code, channel_product_name, channel_price, channel_quantity, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'MAPPED')`
		argsMap := []interface{}{userID, storeID, channelType, match.ProductID, match.ID, channelProductID, channelProductID, skuCode, productName, price, quantity}
		utils.LogSQL(insertMapQuery, argsMap...)
		if _, err := tx.Exec(insertMapQuery, argsMap...); err != nil {
			return err
		}

		return updateWebStock(tx, match.ProductID, match.ID, price, quantity)
	}

	// BƯỚC 3: SKU không trùng
	if autoCreate {
		// Auto create = true -> Tạo mới Sản phẩm trên Web SaaS -> status = MAPPED
		newProductID, newVariantID, err := createNewWebProduct(tx, userID, skuCode, productName, price, quantity)
		if err != nil {
			return err
		}

		insertMapQuery := `
			INSERT INTO channel_product_mappings 
				(user_id, store_id, channel_type, product_id, variant_id, channel_product_id, channel_sku_id, channel_sku_code, channel_product_name, channel_price, channel_quantity, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'MAPPED')`
		argsMap := []interface{}{userID, storeID, channelType, newProductID, newVariantID, channelProductID, channelProductID, skuCode, productName, price, quantity}
		utils.LogSQL(insertMapQuery, argsMap...)
		_, err = tx.Exec(insertMapQuery, argsMap...)
		return err
	} else {
		// Auto create = false -> Lưu vào danh sách chờ syn (product_id & variant_id = NULL, status = UNMAPPED)
		insertUnmapQuery := `
			INSERT INTO channel_product_mappings 
				(user_id, store_id, channel_type, product_id, variant_id, channel_product_id, channel_sku_id, channel_sku_code, channel_product_name, channel_price, channel_quantity, status)
			VALUES (?, ?, ?, NULL, NULL, ?, ?, ?, ?, ?, ?, 'UNMAPPED')`
		argsUnmap := []interface{}{userID, storeID, channelType, channelProductID, channelProductID, skuCode, productName, price, quantity}
		utils.LogSQL(insertUnmapQuery, argsUnmap...)
		_, err := tx.Exec(insertUnmapQuery, argsUnmap...)
		return err
	}
}

// Service Thực hiện Ghép tay thủ công (Manual Map)
func ManualMapProduct(c *gin.Context, mappingID, variantID uint64) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	userID := c.GetUint64("user_id")

	var productID uint64
	queryGetProd := `SELECT product_id FROM product_variants WHERE id = ?`
	utils.LogSQL(queryGetProd, variantID)
	if err := db.Get(&productID, queryGetProd, variantID); err != nil {
		return fmt.Errorf("biến thể sản phẩm không tồn tại")
	}

	updateQuery := `
		UPDATE channel_product_mappings 
		SET product_id = ?, variant_id = ?, status = 'MAPPED', updated_at = NOW()
		WHERE id = ? AND user_id = ? AND status = 'UNMAPPED'`
	utils.LogSQL(updateQuery, productID, variantID, mappingID, userID)

	res, err := db.Exec(updateQuery, productID, variantID, mappingID, userID)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("bản ghi không hợp lệ hoặc đã được ghép trước đó")
	}

	return nil
}

// -----------------------------------------------------------------------------
// HELPER INTERNAL FUNCTIONS
// -----------------------------------------------------------------------------

func updateWebStock(tx *sqlx.Tx, productID, variantID uint64, price float64, qty int) error {
	uQuery := `UPDATE product_variants SET price = ?, quantity = ? WHERE id = ?`
	utils.LogSQL(uQuery, price, qty, variantID)
	if _, err := tx.Exec(uQuery, price, qty, variantID); err != nil {
		return err
	}

	var stockID uint64
	sQuery := `SELECT id FROM manage_stocks WHERE variant_id = ? LIMIT 1`
	utils.LogSQL(sQuery, variantID)
	err := tx.Get(&stockID, sQuery, variantID)

	if err == nil {
		upStock := `UPDATE manage_stocks SET quantity = ? WHERE id = ?`
		utils.LogSQL(upStock, qty, stockID)
		_, err = tx.Exec(upStock, qty, stockID)
		return err
	} else if err == sql.ErrNoRows {
		insStock := `INSERT INTO manage_stocks (warehouse_id, product_id, variant_id, quantity) VALUES (1, ?, ?, ?)`
		utils.LogSQL(insStock, productID, variantID, qty)
		_, err = tx.Exec(insStock, productID, variantID, qty)
		return err
	}
	return err
}

func createNewWebProduct(tx *sqlx.Tx, userID uint64, skuCode, title string, price float64, qty int) (uint64, uint64, error) {
	if title == "" {
		title = "Sản phẩm đồng bộ " + skuCode
	}
	slugStr := fmt.Sprintf("sp-%s-%d", skuCode, time.Now().UnixNano())

	qProd := `INSERT INTO products (user_id, title, slug, active, created_at, updated_at) VALUES (?, ?, ?, 1, NOW(), NOW())`
	utils.LogSQL(qProd, userID, title, slugStr)
	resProd, err := tx.Exec(qProd, userID, title, slugStr)
	if err != nil {
		return 0, 0, err
	}
	pID, _ := resProd.LastInsertId()

	qVar := `INSERT INTO product_variants (product_id, sku, price, quantity, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, 1, NOW(), NOW())`
	utils.LogSQL(qVar, pID, skuCode, price, qty)
	resVar, err := tx.Exec(qVar, pID, skuCode, price, qty)
	if err != nil {
		return 0, 0, err
	}
	vID, _ := resVar.LastInsertId()

	qStock := `INSERT INTO manage_stocks (warehouse_id, product_id, variant_id, quantity, created_at, updated_at) VALUES (1, ?, ?, ?, NOW(), NOW())`
	utils.LogSQL(qStock, pID, vID, qty)
	_, err = tx.Exec(qStock, pID, vID, qty)

	return uint64(pID), uint64(vID), err
}

func getShopeeMockBase(c *gin.Context) string {
	return utils.GetSetting(c, "shopee_mock_base_url", "http://localhost:3003")
}

func ApproveAndCreateProduct(c *gin.Context, mappingID uint64) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("không thể lấy DB từ context: %w", err)
	}

	userID := c.GetUint64("user_id")

	// 1. Kiểm tra bản ghi Mapping xem có tồn tại và đang ở trạng thái UNMAPPED không
	type UnmappedRecord struct {
		ID                 uint64  `db:"id"`
		ChannelSKUCode     string  `db:"channel_sku_code"`
		ChannelProductName string  `db:"channel_product_name"`
		ChannelPrice       float64 `db:"channel_price"`
		ChannelQuantity    int     `db:"channel_quantity"`
	}

	var record UnmappedRecord
	queryGet := `
		SELECT id, channel_sku_code, channel_product_name, channel_price, channel_quantity 
		FROM channel_product_mappings 
		WHERE id = ? AND user_id = ? AND status = 'UNMAPPED'`
	utils.LogSQL(queryGet, mappingID, userID)

	if err := db.Get(&record, queryGet, mappingID, userID); err != nil {
		return fmt.Errorf("bản ghi không tồn tại hoặc đã được xử lý trước đó")
	}

	// 2. Khởi tạo Transaction
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("lỗi khởi tạo transaction: %w", err)
	}
	defer tx.Rollback()

	// 3. Tạo sản phẩm + biến thể + kho mới trên Web
	newProductID, newVariantID, err := createNewWebProduct(
		tx,
		userID,
		record.ChannelSKUCode,
		record.ChannelProductName,
		record.ChannelPrice,
		record.ChannelQuantity,
	)
	if err != nil {
		return fmt.Errorf("lỗi khi tạo sản phẩm mới: %w", err)
	}

	// 4. Cập nhật bản ghi Mapping chuyển từ UNMAPPED -> MAPPED
	updateMapQuery := `
		UPDATE channel_product_mappings 
		SET product_id = ?, variant_id = ?, status = 'MAPPED', updated_at = NOW()
		WHERE id = ? AND user_id = ?`
	utils.LogSQL(updateMapQuery, newProductID, newVariantID, mappingID, userID)

	if _, err := tx.Exec(updateMapQuery, newProductID, newVariantID, mappingID, userID); err != nil {
		return fmt.Errorf("lỗi cập nhật trạng thái mapping: %w", err)
	}

	return tx.Commit()
}

// GetUnmappedProducts lấy danh sách SKU chưa map (dùng chung cho Shopee, TikTok, Lazada...)
func GetUnmappedProducts(c *gin.Context, channelType string, storeID uint64) ([]map[string]interface{}, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	userID := c.GetUint64("user_id")

	query := `
		SELECT id, store_id, channel_type, channel_product_id, channel_sku_id, 
		       channel_sku_code, channel_product_name, channel_price, channel_quantity, channel_image, created_at 
		FROM channel_product_mappings 
		WHERE user_id = ? AND status = 'UNMAPPED'`

	var args []interface{}
	args = append(args, userID)

	if channelType != "" {
		query += " AND channel_type = ?"
		args = append(args, channelType)
	}

	if storeID > 0 {
		query += " AND store_id = ?"
		args = append(args, storeID)
	}

	query += " ORDER BY id DESC"

	utils.LogSQL(query, args...)

	rows, err := db.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		item := make(map[string]interface{})
		if err := rows.MapScan(item); err == nil {
			result = append(result, item)
		}
	}

	return result, nil
}
