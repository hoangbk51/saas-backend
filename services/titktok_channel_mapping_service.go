package services

import (
	"database/sql"
	"fmt"
	"go-saas/models"

	"github.com/jmoiron/sqlx"
)

// ChannelSKUItem DTO tạm thời chứa dữ liệu sản phẩm từ sàn truyền vào Service
type ChannelSKUItem struct {
	StoreID            uint64
	ChannelType        string // 'SHOPEE' hoặc 'TIKTOK'
	ChannelProductID   string
	ChannelSKUID       string
	ChannelSKUCode     string
	ChannelProductName string
	ChannelPrice       float64
	ChannelQuantity    int
	ChannelImage       string
}

// ProcessChannelSKUMapping Tự động xử lý Mapping khi Sync Sản Phẩm từ Sàn
func ProcessChannelSKUMapping(tx *sqlx.Tx, userID uint64, item ChannelSKUItem, autoMap bool) (uint64, uint64, error) {
	var currentMap struct {
		ID        uint64        `db:"id"`
		ProductID sql.NullInt64 `db:"product_id"`
		VariantID sql.NullInt64 `db:"variant_id"`
		Status    string        `db:"status"`
	}

	// 1. Kiểm tra SKU này của sàn đã ghi nhận trong bảng mapping chưa
	checkSQL := "SELECT `id`, `product_id`, `variant_id`, `status` " +
		"FROM `channel_product_mappings` " +
		"WHERE `user_id` = ? AND `channel_type` = ? AND `channel_sku_id` = ? LIMIT 1"

	err := tx.Get(&currentMap, checkSQL, userID, item.ChannelType, item.ChannelSKUID)

	if err == nil && currentMap.Status == "MAPPED" && currentMap.ProductID.Valid && currentMap.VariantID.Valid {
		// Đã MAPPED trước đó -> Chỉ update lại giá, số lượng, tên sản phẩm mới nhất từ sàn
		updateExistSQL := "UPDATE `channel_product_mappings` SET " +
			"`channel_price` = ?, `channel_quantity` = ?, `channel_product_name` = ?, `channel_image` = ?, `updated_at` = NOW() " +
			"WHERE `id` = ?"
		_, _ = tx.Exec(updateExistSQL, item.ChannelPrice, item.ChannelQuantity, item.ChannelProductName, item.ChannelImage, currentMap.ID)

		return uint64(currentMap.ProductID.Int64), uint64(currentMap.VariantID.Int64), nil
	}

	// 2. Chế độ Tự động Match SKU với `product_variants`
	if autoMap && item.ChannelSKUCode != "" {
		var matchedVariant struct {
			ProductID uint64 `db:"product_id"`
			VariantID uint64 `db:"id"`
		}

		findVariantSQL := "SELECT `product_id`, `id` FROM `product_variants` WHERE `user_id` = ? AND `sku` = ? LIMIT 1"
		vErr := tx.Get(&matchedVariant, findVariantSQL, userID, item.ChannelSKUCode)

		if vErr == nil && matchedVariant.VariantID > 0 {
			// Khớp SKU! Upsert thành trạng thái MAPPED
			upsertSQL := "INSERT INTO `channel_product_mappings` (" +
				"`user_id`, `store_id`, `channel_type`, `product_id`, `variant_id`, " +
				"`channel_product_id`, `channel_sku_id`, `channel_sku_code`, `channel_product_name`, " +
				"`channel_price`, `channel_quantity`, `channel_image`, `status`, `created_at`, `updated_at`" +
				") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'MAPPED', NOW(), NOW()) " +
				"ON DUPLICATE KEY UPDATE " +
				"`product_id` = VALUES(`product_id`), `variant_id` = VALUES(`variant_id`), `status` = 'MAPPED', " +
				"`channel_price` = VALUES(`channel_price`), `channel_quantity` = VALUES(`channel_quantity`), `updated_at` = NOW()"

			_, err = tx.Exec(upsertSQL,
				userID, item.StoreID, item.ChannelType, matchedVariant.ProductID, matchedVariant.VariantID,
				item.ChannelProductID, item.ChannelSKUID, item.ChannelSKUCode, item.ChannelProductName,
				item.ChannelPrice, item.ChannelQuantity, item.ChannelImage,
			)

			return matchedVariant.ProductID, matchedVariant.VariantID, err
		}
	}

	// 3. Không map được hoặc không bật AutoMap -> Ghi nhận trạng thái UNMAPPED
	upsertUnmappedSQL := "INSERT INTO `channel_product_mappings` (" +
		"`user_id`, `store_id`, `channel_type`, `product_id`, `variant_id`, " +
		"`channel_product_id`, `channel_sku_id`, `channel_sku_code`, `channel_product_name`, " +
		"`channel_price`, `channel_quantity`, `channel_image`, `status`, `created_at`, `updated_at`" +
		") VALUES (?, ?, ?, NULL, NULL, ?, ?, ?, ?, ?, ?, ?, 'UNMAPPED', NOW(), NOW()) " +
		"ON DUPLICATE KEY UPDATE " +
		"`channel_product_name` = VALUES(`channel_product_name`), `channel_price` = VALUES(`channel_price`), " +
		"`channel_quantity` = VALUES(`channel_quantity`), `updated_at` = NOW()"

	_, err = tx.Exec(upsertUnmappedSQL,
		userID, item.StoreID, item.ChannelType,
		item.ChannelProductID, item.ChannelSKUID, item.ChannelSKUCode, item.ChannelProductName,
		item.ChannelPrice, item.ChannelQuantity, item.ChannelImage,
	)

	return 0, 0, err
}

// GetUnmappedProducts Service lấy danh sách sản phẩm chưa Map (Có thể lọc theo channel_type hoặc store_id)
func GetUnmappedProducts(db *sqlx.DB, userID uint64, channelType string, storeID uint64) ([]map[string]interface{}, error) {
	query := "SELECT `id`, `store_id`, `channel_type`, `channel_product_id`, `channel_sku_id`, " +
		"`channel_sku_code`, `channel_product_name`, `channel_price`, `channel_quantity`, `channel_image`, `created_at` " +
		"FROM `channel_product_mappings` " +
		"WHERE `user_id` = ? AND `status` = 'UNMAPPED' "

	var args []interface{}
	args = append(args, userID)

	if channelType != "" {
		query += "AND `channel_type` = ? "
		args = append(args, channelType)
	}

	if storeID > 0 {
		query += "AND `store_id` = ? "
		args = append(args, storeID)
	}

	query += "ORDER BY `id` DESC"

	rows, err := db.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		item := make(map[string]interface{})
		if err := rows.MapScan(item); err == nil {
			results = append(results, item)
		}
	}
	return results, nil
}

// ManualMapSKU Service ghép nối thủ công
func ManualMapSKU(db *sqlx.DB, userID uint64, req models.ManualMapSKURequest) error {
	query := "UPDATE `channel_product_mappings` SET " +
		"`product_id` = ?, `variant_id` = ?, `status` = 'MAPPED', " +
		"`sync_inventory` = ?, `sync_price` = ?, `updated_at` = NOW() " +
		"WHERE `user_id` = ? AND `channel_type` = ? AND `channel_sku_id` = ?"

	res, err := db.Exec(query,
		req.ProductID, req.VariantID, req.SyncInventory, req.SyncPrice,
		userID, req.ChannelType, req.ChannelSKUID,
	)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("không tìm thấy dữ liệu SKU cần ghép nối")
	}
	return nil
}
