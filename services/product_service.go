package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Hàm đồng bộ thông tin giảm giá mua nhiều vào DB
func SyncProductDiscounts(c *gin.Context, tx *sql.Tx, productID int64, discounts []models.ProductDiscount) error {
	// 1. Xóa toàn bộ giảm giá cũ của sản phẩm
	deleteQuery := `DELETE FROM product_discounts WHERE product_id = ?`
	if _, err := tx.ExecContext(c.Request.Context(), deleteQuery, productID); err != nil {
		return fmt.Errorf("lỗi xóa giảm giá cũ: %v", err)
	}

	if len(discounts) == 0 {
		return nil
	}

	// 2. Insert lại danh sách giảm giá mới
	insertQuery := `INSERT INTO product_discounts 
		(product_id, quantity, priority, price, date_start, date_end, created_at, updated_at) 
		VALUES `

	var vals []interface{}
	for _, d := range discounts {
		insertQuery += "(?, ?, ?, ?, ?, ?, NOW(), NOW()),"

		var dateStart, dateEnd interface{}
		if d.DateStart != nil && *d.DateStart != "" {
			dateStart = *d.DateStart
		}
		if d.DateEnd != nil && *d.DateEnd != "" {
			dateEnd = *d.DateEnd
		}

		vals = append(vals, productID, d.Quantity, d.Priority, d.Price, dateStart, dateEnd)
	}

	// Cắt bỏ dấu phẩy cuối cùng
	insertQuery = insertQuery[:len(insertQuery)-1]

	if _, err := tx.ExecContext(c.Request.Context(), insertQuery, vals...); err != nil {
		return fmt.Errorf("lỗi thêm mới danh sách giảm giá: %v", err)
	}

	return nil
}

func CreateProduct(c *gin.Context, p *models.ProductSaveRequest) (int64, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, err
	}

	if err := c.ShouldBind(p); err != nil {
		return 0, fmt.Errorf("lỗi bind form data sản phẩm: %v", err)
	}

	p.Title = make(map[string]string)
	p.Description = make(map[string]string)
	p.ShortDescription = make(map[string]string)
	p.MetaTitle = make(map[string]string)
	p.MetaDescription = make(map[string]string)

	if titleRaw := c.PostForm("title"); titleRaw != "" {
		_ = json.Unmarshal([]byte(titleRaw), &p.Title)
	}
	if descRaw := c.PostForm("description"); descRaw != "" {
		_ = json.Unmarshal([]byte(descRaw), &p.Description)
	}
	if shortDescRaw := c.PostForm("short_description"); shortDescRaw != "" {
		_ = json.Unmarshal([]byte(shortDescRaw), &p.ShortDescription)
	}
	if metaTitleRaw := c.PostForm("meta_title"); metaTitleRaw != "" {
		_ = json.Unmarshal([]byte(metaTitleRaw), &p.MetaTitle)
	}
	if metaDescRaw := c.PostForm("meta_description"); metaDescRaw != "" {
		_ = json.Unmarshal([]byte(metaDescRaw), &p.MetaDescription)
	}
	if discountsRaw := c.PostForm("discounts"); discountsRaw != "" {
		_ = json.Unmarshal([]byte(discountsRaw), &p.Discounts)
	}

	if p.Slug == "" {
		if viTitle, ok := p.Title["vi"]; ok && viTitle != "" {
			p.Slug = utils.GenerateSlug(viTitle)
		} else if enTitle, ok := p.Title["en"]; ok && enTitle != "" {
			p.Slug = utils.GenerateSlug(enTitle)
		} else {
			p.Slug = fmt.Sprintf("product-%d", time.Now().UnixNano())
		}
	}

	relatedJSON, err := json.Marshal(p.RelatedProducts)
	if err != nil {
		return 0, fmt.Errorf("lỗi encode related products: %v", err)
	}

	titleJSON, _ := json.Marshal(p.Title)
	descriptionJSON, _ := json.Marshal(p.Description)
	shortDescJSON, _ := json.Marshal(p.ShortDescription)
	metaTitleJSON, _ := json.Marshal(p.MetaTitle)
	metaDescJSON, _ := json.Marshal(p.MetaDescription)

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return 0, fmt.Errorf("không thể khởi tạo transaction: %v", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO products (
				brand_id, title, model_number, sku, ` + "`condition`" + `, 
				stock_quantity, purchase_price, sale_price, shipping_weight, 
				active, slug, description, short_description, meta_title, 
				meta_description, link_video, linked_items, created_at, updated_at
			  ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	utils.LogSQL(query,
		p.BrandID, string(titleJSON), p.ModelNumber, p.SKU, p.Condition,
		p.StockQuantity, p.PurchasePrice, p.SalePrice, p.ShippingWeight,
		p.Active, p.Slug, string(descriptionJSON), string(shortDescJSON),
		string(metaTitleJSON), string(metaDescJSON), p.LinkVideo, string(relatedJSON),
	)

	result, err := tx.ExecContext(c.Request.Context(), query,
		p.BrandID, titleJSON, p.ModelNumber, p.SKU, p.Condition,
		p.StockQuantity, p.PurchasePrice, p.SalePrice, p.ShippingWeight,
		p.Active, p.Slug, descriptionJSON, shortDescJSON, metaTitleJSON,
		metaDescJSON, p.LinkVideo, relatedJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("lỗi insert db product: %v", err)
	}

	productID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("không thể lấy id sản phẩm mới tạo: %v", err)
	}

	if len(p.CategoryIDs) > 0 {
		insertCategoriesQuery := `INSERT INTO category_product (product_id, category_id) VALUES `
		var vals []interface{}
		for _, catID := range p.CategoryIDs {
			insertCategoriesQuery += "(?, ?),"
			vals = append(vals, productID, catID)
		}
		insertCategoriesQuery = insertCategoriesQuery[:len(insertCategoriesQuery)-1]

		_, err = tx.ExecContext(c.Request.Context(), insertCategoriesQuery, vals...)
		if err != nil {
			return 0, fmt.Errorf("lỗi lưu liên kết danh mục mới: %v", err)
		}
	}

	if len(p.Attributes) > 0 {
		err = SyncProductAttributes(c, tx, uint64(productID), p.Attributes)
		if err != nil {
			return 0, fmt.Errorf("lỗi đồng bộ thuộc tính: %v", err)
		}
	}

	if len(p.Variants) > 0 {
		if err := SyncProductVariants(c, tx, uint64(productID), p.Variants); err != nil {
			return 0, fmt.Errorf("lỗi đồng bộ biến thể sản phẩm: %v", err)
		}
	} else {

		// TH 2: KHÔNG CÓ VARIANT -> Tự động tạo 1 Record Mặc Định vào product_variants
		defaultSKU := p.SKU
		if defaultSKU == "" {
			defaultSKU = fmt.Sprintf("SKU-%d", productID) // Fallback SKU nếu sản phẩm chính không nhập SKU
		}

		queryDefaultVariant := `
			INSERT INTO product_variants (
				product_id, sku, price, compare_at_price, quantity, image, is_default, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, 1, NOW(), NOW())
		`
		_, err := tx.ExecContext(c, queryDefaultVariant,
			productID, defaultSKU, p.SalePrice, p.SalePrice, p.StockQuantity, p.Image,
		)
		if err != nil {
			return 0, fmt.Errorf("Lỗi tạo biến thể mặc định: %v", err)
		}
	}

	// Đồng bộ danh sách giảm giá sản phẩm
	if len(p.Discounts) > 0 {
		if err := SyncProductDiscounts(c, tx, productID, p.Discounts); err != nil {
			return 0, fmt.Errorf("lỗi đồng bộ giảm giá sản phẩm: %v", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("lỗi commit transaction: %v", err)
	}

	_, _ = HandleFileUpload(c, UploadParam{
		ModelType:      "App\\Models\\Tenant\\Product",
		ModelID:        productID,
		CollectionName: "main_images",
		FieldName:      "image",
	})

	_, _ = HandleFileUpload(c, UploadParam{
		ModelType:      "App\\Models\\Tenant\\Product",
		ModelID:        productID,
		CollectionName: "sub_images",
		FieldName:      "images",
	})

	return productID, nil
}

func UpdateProduct(c *gin.Context, id string, p *models.ProductSaveRequest) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}
	productID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("ID không hợp lệ: %v", err)
	}

	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		parseJSONForm := func(key string, target interface{}) {
			if val := c.PostForm(key); val != "" {
				_ = json.Unmarshal([]byte(val), target)
			}
		}

		parseJSONForm("title", &p.Title)
		parseJSONForm("description", &p.Description)
		parseJSONForm("short_description", &p.ShortDescription)
		parseJSONForm("meta_title", &p.MetaTitle)
		parseJSONForm("meta_description", &p.MetaDescription)
		parseJSONForm("variants", &p.Variants)
		parseJSONForm("attributes", &p.Attributes)
		parseJSONForm("category_ids", &p.CategoryIDs)
		parseJSONForm("related_products", &p.RelatedProducts)
		parseJSONForm("delete_sub_image_ids", &p.DeleteSubImageIDs)
		parseJSONForm("discounts", &p.Discounts)
	}

	if p.RelatedProducts == nil {
		p.RelatedProducts = []uint64{}
	}

	relatedJSON, _ := json.Marshal(p.RelatedProducts)
	titleJSON, _ := json.Marshal(p.Title)
	descriptionJSON, _ := json.Marshal(p.Description)
	shortDescJSON, _ := json.Marshal(p.ShortDescription)
	metaTitleJSON, _ := json.Marshal(p.MetaTitle)
	metaDescJSON, _ := json.Marshal(p.MetaDescription)

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return fmt.Errorf("không thể khởi tạo transaction: %v", err)
	}
	defer tx.Rollback()

	if len(p.DeleteSubImageIDs) > 0 {
		deleteMediaStr := `DELETE FROM media WHERE id = ? AND model_id = ? AND collection_name = 'sub_images' AND model_type = 'App\\Models\\Tenant\\Product'`
		for _, mediaID := range p.DeleteSubImageIDs {
			_, err := tx.ExecContext(c.Request.Context(), deleteMediaStr, mediaID, productID)
			if err != nil {
				return fmt.Errorf("lỗi khi xóa ảnh phụ ID %d: %v", mediaID, err)
			}
		}
	}

	query := `UPDATE products SET 
				brand_id = ?, title = ?, model_number = ?, sku = ?, ` + "`condition`" + ` = ?, 
				stock_quantity = ?, purchase_price = ?, sale_price = ?, shipping_weight = ?, 
				active = ?, slug = ?, description = ?, short_description = ?, meta_title = ?, 
				meta_description = ?, link_video = ?, linked_items = ?, updated_at = NOW()
			  WHERE id = ? AND deleted_at IS NULL`

	res, err := tx.ExecContext(c.Request.Context(), query,
		p.BrandID, titleJSON, p.ModelNumber, p.SKU, p.Condition,
		p.StockQuantity, p.PurchasePrice, p.SalePrice, p.ShippingWeight,
		p.Active, p.Slug, descriptionJSON, shortDescJSON, metaTitleJSON,
		metaDescJSON, p.LinkVideo, relatedJSON, productID,
	)
	if err != nil {
		return fmt.Errorf("lỗi cập nhật bảng products: %v", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("sản phẩm có ID %d không tồn tại hoặc đã bị xóa mềm", productID)
	}

	if len(p.CategoryIDs) > 0 {
		deleteOldCats := `DELETE FROM category_product WHERE product_id = ?`
		_, err = tx.ExecContext(c.Request.Context(), deleteOldCats, productID)
		if err != nil {
			return fmt.Errorf("lỗi làm sạch danh mục cũ: %v", err)
		}

		insertCategoriesQuery := `INSERT INTO category_product (product_id, category_id) VALUES `
		var vals []interface{}
		for _, catID := range p.CategoryIDs {
			insertCategoriesQuery += "(?, ?),"
			vals = append(vals, productID, catID)
		}
		insertCategoriesQuery = insertCategoriesQuery[:len(insertCategoriesQuery)-1]

		_, err = tx.ExecContext(c.Request.Context(), insertCategoriesQuery, vals...)
		if err != nil {
			return fmt.Errorf("lỗi cập nhật liên kết danh mục: %v", err)
		}
	}

	if p.Attributes != nil {
		if err := SyncProductAttributes(c, tx, uint64(productID), p.Attributes); err != nil {
			return fmt.Errorf("lỗi đồng bộ thuộc tính: %v", err)
		}
	}

	if p.Variants != nil {
		if err := SyncProductVariants(c, tx, uint64(productID), p.Variants); err != nil {
			return fmt.Errorf("lỗi cập nhật biến thể sản phẩm: %v", err)
		}
	}

	// Cập nhật danh sách giảm giá sản phẩm
	if p.Discounts != nil {
		if err := SyncProductDiscounts(c, tx, productID, p.Discounts); err != nil {
			return fmt.Errorf("lỗi đồng bộ giảm giá sản phẩm: %v", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("lỗi commit transaction update: %v", err)
	}

	_, _ = HandleFileUpload(c, UploadParam{
		ModelType:      "App\\Models\\Tenant\\Product",
		ModelID:        productID,
		CollectionName: "main_images",
		FieldName:      "image",
	})

	_, _ = HandleFileUpload(c, UploadParam{
		ModelType:      "App\\Models\\Tenant\\Product",
		ModelID:        productID,
		CollectionName: "sub_images",
		FieldName:      "images",
	})

	return nil
}

func DeleteProduct(c *gin.Context, id string) error {
	productID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("ID không hợp lệ: %v", err)
	}

	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return fmt.Errorf("không thể khởi tạo transaction: %v", err)
	}
	defer tx.Rollback()

	// 1. Xóa các liên kết danh mục
	deleteCategoriesQuery := `DELETE FROM category_product WHERE product_id = ?`
	if _, err = tx.ExecContext(c.Request.Context(), deleteCategoriesQuery, productID); err != nil {
		return fmt.Errorf("lỗi xóa liên kết danh mục: %v", err)
	}

	// 2. Xóa thông tin giảm giá mua nhiều
	deleteDiscountsQuery := `DELETE FROM product_discounts WHERE product_id = ?`
	if _, err = tx.ExecContext(c.Request.Context(), deleteDiscountsQuery, productID); err != nil {
		return fmt.Errorf("lỗi xóa giảm giá sản phẩm: %v", err)
	}

	// 3. Xóa sản phẩm
	deleteProductQuery := `DELETE FROM products WHERE id = ?`
	result, err := tx.ExecContext(c.Request.Context(), deleteProductQuery, productID)
	if err != nil {
		return fmt.Errorf("lỗi xóa sản phẩm trong DB: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("sản phẩm có ID %d không tồn tại", productID)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("lỗi commit transaction xóa sản phẩm: %v", err)
	}

	return nil
}

func GetProductDetail(c *gin.Context, productID int64) (*models.Product, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	domainApi := c.Request.Host
	tenantId, _ := c.Get("tenantId")

	p := &models.Product{}

	query := `
		SELECT 
			p.id, 
			p.title, 
			p.slug,
			COALESCE(p.sale_price, 0) as price, 
			COALESCE(p.brand_id, 0) as brand_id,
			COALESCE(p.model_number, '') as model_number,
			COALESCE(p.purchase_price, 0) as purchase_price,
			COALESCE(p.sku, '') as sku,
			p.stock_quantity, 
			COALESCE(p.offer_price, 0),
			p.description,
			p.short_description,
			p.meta_title,
			p.meta_description,
			COALESCE(p.shipping_weight, 0),
			COALESCE(p.try_on_status, 0),
			p.rating,
			p.offer_start,
			p.offer_end,
			p.linked_items,
			m.file_name, 
			m.id as model_id
		FROM products p
		LEFT JOIN media m ON m.id = (
			SELECT MAX(id) 
			FROM media 
			WHERE model_id = p.id 
			AND model_type = 'App\\Models\\Tenant\\Product' 
			AND collection_name = 'main_images'
		)
		WHERE p.id = ? AND p.deleted_at IS NULL
		LIMIT 1
	`

	var (
		titleRaw       []byte
		descRaw        []byte
		shortDescRaw   []byte
		metaTitleRaw   []byte
		metaDescRaw    []byte
		linkedItemsRaw []byte
	)
	var offerStartNull, offerEndNull sql.NullTime

	err = db.QueryRow(query, productID).Scan(
		&p.ID,
		&titleRaw,
		&p.Slug,
		&p.SalePrice,
		&p.BrandID,
		&p.ModelNumber,
		&p.PurchasePrice,
		&p.SKU,
		&p.StockQuantity,
		&p.OfferPrice,
		&descRaw,
		&shortDescRaw,
		&metaTitleRaw,
		&metaDescRaw,
		&p.ShippingWeight,
		&p.TryOnStatus,
		&p.Rate,
		&offerStartNull,
		&offerEndNull,
		&linkedItemsRaw,
		&p.FileName,
		&p.ModelID,
	)

	if err != nil {
		return nil, err
	}

	p.Title = make(map[string]string)
	p.Description = make(map[string]string)
	p.ShortDescription = make(map[string]string)
	p.MetaTitle = make(map[string]string)
	p.MetaDescription = make(map[string]string)
	p.RelatedProducts = []uint64{}

	if len(titleRaw) > 0 {
		_ = json.Unmarshal(titleRaw, &p.Title)
	}
	if len(descRaw) > 0 {
		_ = json.Unmarshal(descRaw, &p.Description)
	}
	if len(shortDescRaw) > 0 {
		_ = json.Unmarshal(shortDescRaw, &p.ShortDescription)
	}
	if len(metaTitleRaw) > 0 {
		_ = json.Unmarshal(metaTitleRaw, &p.MetaTitle)
	}
	if len(metaDescRaw) > 0 {
		_ = json.Unmarshal(metaDescRaw, &p.MetaDescription)
	}
	if len(linkedItemsRaw) > 0 {
		_ = json.Unmarshal(linkedItemsRaw, &p.RelatedProducts)
	}

	if offerStartNull.Valid {
		p.OfferStart = offerStartNull.Time
	}
	if offerEndNull.Valid {
		p.OfferEnd = offerEndNull.Time
	}

	if p.ModelID.Valid && p.FileName.Valid && p.FileName.String != "" {
		p.Image = fmt.Sprintf("http://%s/storage/tenancy/%s/app/public/%v/%v",
			domainApi, tenantId, p.ModelID.Int64, p.FileName.String,
		)
	} else {
		p.Image = "https://tutaoweb.com/images/clothe.png"
	}

	p.SubImages = []models.ProductImage{}
	subImagesQuery := `
		SELECT id, file_name 
		FROM media 
		WHERE model_id = ? 
		AND collection_name = 'sub_images' 
		AND model_type = 'App\\Models\\Tenant\\Product'
		ORDER BY id DESC
	`
	rows, err := db.Query(subImagesQuery, productID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var mID int64
			var fName string
			if err := rows.Scan(&mID, &fName); err == nil {
				fullPath := fmt.Sprintf("http://%s/storage/tenancy/%s/app/public/%v/%v",
					domainApi, tenantId, mID, fName)

				p.SubImages = append(p.SubImages, models.ProductImage{
					ID:  mID,
					URL: fullPath,
				})
			}
		}
	}

	p.Categories = []models.Category{}
	categoryQuery := `
		SELECT 
			c.id, 
			c.parent_id, 
			COALESCE(c.name, '') as name, 
			COALESCE(c.slug, '') as slug, 
			COALESCE(c.description, '') as description
		FROM categories c
		INNER JOIN category_product cp ON c.id = cp.category_id
		WHERE cp.product_id = ? AND c.deleted_at IS NULL
	`
	catRows, err := db.Query(categoryQuery, productID)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat models.Category
			var parentIDNull sql.NullInt64

			err := catRows.Scan(&cat.ID, &parentIDNull, &cat.Name, &cat.Slug, &cat.Description)
			if err == nil {
				if parentIDNull.Valid {
					parentVal := uint32(parentIDNull.Int64)
					cat.ParentID = &parentVal
				} else {
					cat.ParentID = nil
				}
				p.Categories = append(p.Categories, cat)
			}
		}
	}

	now := time.Now()
	p.HasOffer = false
	p.DiscountPercentage = 0.0
	const timeLayout = "2006-01-02 15:04:05"

	if !p.OfferStart.IsZero() {
		p.OfferStartStr = p.OfferStart.Format(timeLayout)
	}
	if !p.OfferEnd.IsZero() {
		p.OfferEndStr = p.OfferEnd.Format(timeLayout)
	}

	p.FinalPrice = p.SalePrice
	if p.OfferPrice > 0 && p.OfferPrice < p.SalePrice {
		isStarted := p.OfferStart.IsZero() || now.After(p.OfferStart) || now.Equal(p.OfferStart)
		isNotExpired := p.OfferEnd.IsZero() || now.Before(p.OfferEnd) || now.Equal(p.OfferEnd)

		if isStarted && isNotExpired {
			p.HasOffer = true
			p.FinalPrice = p.OfferPrice
			p.DiscountPercentage = math.Round(((p.SalePrice - p.OfferPrice) / p.SalePrice) * 100)
		}
	}

	attrQuery := `
		SELECT 
			pa.id AS product_attribute_id,
			pa.attribute_id,
			COALESCE(a.name, '') AS attribute_name,
			pa.attribute_value_id,
			av.name AS value_name
		FROM product_attribute pa
		LEFT JOIN attributes a ON a.id = pa.attribute_id
		LEFT JOIN attribute_values av ON av.id = pa.attribute_value_id AND av.deleted_at IS NULL
		WHERE pa.product_id = ?
		ORDER BY pa.attribute_id ASC, pa.id ASC
	`

	attrRows, err := db.QueryContext(c.Request.Context(), attrQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn attributes: %v", err)
	}
	defer attrRows.Close()

	attrGroupMap := make(map[uint32]*models.GroupedAttribute)
	attrOrders := make([]uint32, 0)

	for attrRows.Next() {
		var (
			productAttributeID uint64
			attributeID        uint32
			attributeName      string
			attributeValueID   int
			valueName          sql.NullString
		)

		err := attrRows.Scan(
			&productAttributeID,
			&attributeID,
			&attributeName,
			&attributeValueID,
			&valueName,
		)
		if err != nil {
			continue
		}

		var valNameStr *string
		if valueName.Valid {
			str := valueName.String
			valNameStr = &str
		}

		if _, exists := attrGroupMap[attributeID]; !exists {
			attrGroupMap[attributeID] = &models.GroupedAttribute{
				AttributeID:   attributeID,
				AttributeName: attributeName,
				Items:         make([]models.AttributeValueItem, 0),
			}
			attrOrders = append(attrOrders, attributeID)
		}

		attrGroupMap[attributeID].Items = append(attrGroupMap[attributeID].Items, models.AttributeValueItem{
			ProductAttributeID: productAttributeID,
			AttributeValueID:   attributeValueID,
			ValueName:          valNameStr,
		})
	}

	p.Attributes = make([]models.GroupedAttribute, 0, len(attrOrders))
	for _, aID := range attrOrders {
		p.Attributes = append(p.Attributes, *attrGroupMap[aID])
	}

	variantsQuery := `
		SELECT 
			v.id, v.sku, v.price, v.compare_at_price, v.quantity, COALESCE(v.image, ''), v.is_default,
			COALESCE(GROUP_CONCAT(pvov.option_value_id), '') AS option_value_ids
		FROM product_variants v
		LEFT JOIN product_variant_option_values pvov ON v.id = pvov.variant_id
		WHERE v.product_id = ?
		GROUP BY v.id
		ORDER BY v.id ASC
	`

	vRows, err := db.QueryContext(c.Request.Context(), variantsQuery, productID)
	if err == nil {
		defer vRows.Close()
		var variants []models.ProductVariantDetail

		for vRows.Next() {
			var (
				vDetail       models.ProductVariantDetail
				ovIDsStr      string
				compPriceNull sql.NullFloat64
			)

			err := vRows.Scan(
				&vDetail.ID,
				&vDetail.SKU,
				&vDetail.Price,
				&compPriceNull,
				&vDetail.Quantity,
				&vDetail.Image,
				&vDetail.IsDefault,
				&ovIDsStr,
			)
			if err != nil {
				continue
			}

			if compPriceNull.Valid {
				vDetail.CompareAtPrice = &compPriceNull.Float64
			}

			vDetail.OptionValueIDs = []uint64{}
			if ovIDsStr != "" {
				parts := strings.Split(ovIDsStr, ",")
				for _, p := range parts {
					if val, err := strconv.ParseUint(p, 10, 64); err == nil {
						vDetail.OptionValueIDs = append(vDetail.OptionValueIDs, val)
					}
				}
			}

			variants = append(variants, vDetail)
		}

		p.Variants = variants
	}

	// --- BỔ SUNG: Lấy danh sách giảm giá sản phẩm ---
	p.Discounts = []models.ProductDiscount{}
	discountsQuery := `
		SELECT id, product_id, quantity, priority, price, date_start, date_end
		FROM product_discounts
		WHERE product_id = ?
		ORDER BY priority ASC, quantity ASC
	`
	dRows, err := db.QueryContext(c.Request.Context(), discountsQuery, productID)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d models.ProductDiscount
			var dStartNull, dEndNull sql.NullString

			err := dRows.Scan(&d.ID, &d.ProductID, &d.Quantity, &d.Priority, &d.Price, &dStartNull, &dEndNull)
			if err == nil {
				if dStartNull.Valid {
					d.DateStart = &dStartNull.String
				}
				if dEndNull.Valid {
					d.DateEnd = &dEndNull.String
				}
				p.Discounts = append(p.Discounts, d)
			}
		}
	}

	return p, nil
}

func GetProductBySlug(c *gin.Context, slug string) (*models.Product, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	domainApi := c.Request.Host
	tenantId, _ := c.Get("tenantId")

	p := &models.Product{}

	query := `
		SELECT 
			p.id, 
			p.title, 
			p.slug,
			COALESCE(p.sale_price, 0) as price, 
			COALESCE(p.brand_id, 0) as brand_id,
			COALESCE(p.model_number, '') as model_number,
			COALESCE(p.purchase_price, 0) as purchase_price,
			COALESCE(p.sku, '') as sku,
			p.stock_quantity, 
			COALESCE(p.offer_price, 0),
			p.description,
			p.short_description,
			p.meta_title,
			p.meta_description,
			COALESCE(p.shipping_weight, 0),
			COALESCE(p.try_on_status, 0),
			p.rating,
			p.offer_start,
			p.offer_end,
			p.linked_items,
			m.file_name, 
			m.id as model_id,
			m.url
		FROM products p
		LEFT JOIN media m ON m.id = (
			SELECT MAX(id) 
			FROM media 
			WHERE model_id = p.id 
			AND model_type = 'App\\Models\\Tenant\\Product' 
			AND collection_name = 'main_images'
		)
		WHERE p.slug = ? AND p.deleted_at IS NULL
		LIMIT 1
	`

	var (
		titleRaw       []byte
		descRaw        []byte
		shortDescRaw   []byte
		metaTitleRaw   []byte
		metaDescRaw    []byte
		linkedItemsRaw []byte
	)
	var offerStartNull, offerEndNull sql.NullTime

	err = db.QueryRow(query, slug).Scan(
		&p.ID,
		&titleRaw,
		&p.Slug,
		&p.SalePrice,
		&p.BrandID,
		&p.ModelNumber,
		&p.PurchasePrice,
		&p.SKU,
		&p.StockQuantity,
		&p.OfferPrice,
		&descRaw,
		&shortDescRaw,
		&metaTitleRaw,
		&metaDescRaw,
		&p.ShippingWeight,
		&p.TryOnStatus,
		&p.Rate,
		&offerStartNull,
		&offerEndNull,
		&linkedItemsRaw,
		&p.FileName,
		&p.ModelID,
		&p.Image,
	)

	if err != nil {
		return nil, err
	}
	productID := p.ID
	p.Title = make(map[string]string)
	p.Description = make(map[string]string)
	p.ShortDescription = make(map[string]string)
	p.MetaTitle = make(map[string]string)
	p.MetaDescription = make(map[string]string)
	p.RelatedProducts = []uint64{}

	if len(titleRaw) > 0 {
		_ = json.Unmarshal(titleRaw, &p.Title)
	}
	if len(descRaw) > 0 {
		_ = json.Unmarshal(descRaw, &p.Description)
	}
	if len(shortDescRaw) > 0 {
		_ = json.Unmarshal(shortDescRaw, &p.ShortDescription)
	}
	if len(metaTitleRaw) > 0 {
		_ = json.Unmarshal(metaTitleRaw, &p.MetaTitle)
	}
	if len(metaDescRaw) > 0 {
		_ = json.Unmarshal(metaDescRaw, &p.MetaDescription)
	}
	if len(linkedItemsRaw) > 0 {
		_ = json.Unmarshal(linkedItemsRaw, &p.RelatedProducts)
	}

	if offerStartNull.Valid {
		p.OfferStart = offerStartNull.Time
	}
	if offerEndNull.Valid {
		p.OfferEnd = offerEndNull.Time
	}

	p.SubImages = []models.ProductImage{}
	subImagesQuery := `
		SELECT id, file_name 
		FROM media 
		WHERE model_id = ? 
		AND collection_name = 'sub_images' 
		AND model_type = 'App\\Models\\Tenant\\Product'
		ORDER BY id DESC
	`
	rows, err := db.Query(subImagesQuery, productID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var mID int64
			var fName string
			if err := rows.Scan(&mID, &fName); err == nil {
				fullPath := fmt.Sprintf("http://%s/storage/tenancy/%s/app/public/%v/%v",
					domainApi, tenantId, mID, fName)

				p.SubImages = append(p.SubImages, models.ProductImage{
					ID:  mID,
					URL: fullPath,
				})
			}
		}
	}

	p.Categories = []models.Category{}
	categoryQuery := `
		SELECT 
			c.id, 
			c.parent_id, 
			COALESCE(c.name, '') as name, 
			COALESCE(c.slug, '') as slug, 
			COALESCE(c.description, '') as description
		FROM categories c
		INNER JOIN category_product cp ON c.id = cp.category_id
		WHERE cp.product_id = ? AND c.deleted_at IS NULL
	`
	catRows, err := db.Query(categoryQuery, productID)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat models.Category
			var parentIDNull sql.NullInt64

			err := catRows.Scan(&cat.ID, &parentIDNull, &cat.Name, &cat.Slug, &cat.Description)
			if err == nil {
				if parentIDNull.Valid {
					parentVal := uint32(parentIDNull.Int64)
					cat.ParentID = &parentVal
				} else {
					cat.ParentID = nil
				}
				p.Categories = append(p.Categories, cat)
			}
		}
	}

	now := time.Now()
	p.HasOffer = false
	p.DiscountPercentage = 0.0
	const timeLayout = "2006-01-02 15:04:05"

	if !p.OfferStart.IsZero() {
		p.OfferStartStr = p.OfferStart.Format(timeLayout)
	}
	if !p.OfferEnd.IsZero() {
		p.OfferEndStr = p.OfferEnd.Format(timeLayout)
	}

	p.FinalPrice = p.SalePrice
	if p.OfferPrice > 0 && p.OfferPrice < p.SalePrice {
		isStarted := p.OfferStart.IsZero() || now.After(p.OfferStart) || now.Equal(p.OfferStart)
		isNotExpired := p.OfferEnd.IsZero() || now.Before(p.OfferEnd) || now.Equal(p.OfferEnd)

		if isStarted && isNotExpired {
			p.HasOffer = true
			p.FinalPrice = p.OfferPrice
			p.DiscountPercentage = math.Round(((p.SalePrice - p.OfferPrice) / p.SalePrice) * 100)
		}
	}

	attrQuery := `
		SELECT 
			pa.id AS product_attribute_id,
			pa.attribute_id,
			COALESCE(a.name, '') AS attribute_name,
			pa.attribute_value_id,
			av.name AS value_name
		FROM product_attribute pa
		LEFT JOIN attributes a ON a.id = pa.attribute_id
		LEFT JOIN attribute_values av ON av.id = pa.attribute_value_id AND av.deleted_at IS NULL
		WHERE pa.product_id = ?
		ORDER BY pa.attribute_id ASC, pa.id ASC
	`

	attrRows, err := db.QueryContext(c.Request.Context(), attrQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn attributes: %v", err)
	}
	defer attrRows.Close()

	attrGroupMap := make(map[uint32]*models.GroupedAttribute)
	attrOrders := make([]uint32, 0)

	for attrRows.Next() {
		var (
			productAttributeID uint64
			attributeID        uint32
			attributeName      string
			attributeValueID   int
			valueName          sql.NullString
		)

		err := attrRows.Scan(
			&productAttributeID,
			&attributeID,
			&attributeName,
			&attributeValueID,
			&valueName,
		)
		if err != nil {
			continue
		}

		var valNameStr *string
		if valueName.Valid {
			str := valueName.String
			valNameStr = &str
		}

		if _, exists := attrGroupMap[attributeID]; !exists {
			attrGroupMap[attributeID] = &models.GroupedAttribute{
				AttributeID:   attributeID,
				AttributeName: attributeName,
				Items:         make([]models.AttributeValueItem, 0),
			}
			attrOrders = append(attrOrders, attributeID)
		}

		attrGroupMap[attributeID].Items = append(attrGroupMap[attributeID].Items, models.AttributeValueItem{
			ProductAttributeID: productAttributeID,
			AttributeValueID:   attributeValueID,
			ValueName:          valNameStr,
		})
	}

	p.Attributes = make([]models.GroupedAttribute, 0, len(attrOrders))
	for _, aID := range attrOrders {
		p.Attributes = append(p.Attributes, *attrGroupMap[aID])
	}

	variantsQuery := `
		SELECT 
			v.id, v.sku, v.price, v.compare_at_price, v.quantity, COALESCE(v.image, ''), v.is_default,
			COALESCE(GROUP_CONCAT(pvov.option_value_id), '') AS option_value_ids
		FROM product_variants v
		LEFT JOIN product_variant_option_values pvov ON v.id = pvov.variant_id
		WHERE v.product_id = ?
		GROUP BY v.id
		ORDER BY v.id ASC
	`

	vRows, err := db.QueryContext(c.Request.Context(), variantsQuery, productID)
	if err == nil {
		defer vRows.Close()
		var variants []models.ProductVariantDetail

		for vRows.Next() {
			var (
				vDetail       models.ProductVariantDetail
				ovIDsStr      string
				compPriceNull sql.NullFloat64
			)

			err := vRows.Scan(
				&vDetail.ID,
				&vDetail.SKU,
				&vDetail.Price,
				&compPriceNull,
				&vDetail.Quantity,
				&vDetail.Image,
				&vDetail.IsDefault,
				&ovIDsStr,
			)
			if err != nil {
				continue
			}

			if compPriceNull.Valid {
				vDetail.CompareAtPrice = &compPriceNull.Float64
			}

			vDetail.OptionValueIDs = []uint64{}
			if ovIDsStr != "" {
				parts := strings.Split(ovIDsStr, ",")
				for _, p := range parts {
					if val, err := strconv.ParseUint(p, 10, 64); err == nil {
						vDetail.OptionValueIDs = append(vDetail.OptionValueIDs, val)
					}
				}
			}

			variants = append(variants, vDetail)
		}

		p.Variants = variants
	}

	// --- BỔ SUNG: Lấy danh sách giảm giá sản phẩm ---
	p.Discounts = []models.ProductDiscount{}
	discountsQuery := `
		SELECT id, product_id, quantity, priority, price, date_start, date_end
		FROM product_discounts
		WHERE product_id = ?
		ORDER BY priority ASC, quantity ASC
	`
	dRows, err := db.QueryContext(c.Request.Context(), discountsQuery, productID)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var d models.ProductDiscount
			var dStartNull, dEndNull sql.NullString

			err := dRows.Scan(&d.ID, &d.ProductID, &d.Quantity, &d.Priority, &d.Price, &dStartNull, &dEndNull)
			if err == nil {
				if dStartNull.Valid {
					d.DateStart = &dStartNull.String
				}
				if dEndNull.Valid {
					d.DateEnd = &dEndNull.String
				}
				p.Discounts = append(p.Discounts, d)
			}
		}
	}

	return p, nil
}

// -----------------------------------------------------------------------------
// Các hàm trợ giúp xử lý Attribute
// -----------------------------------------------------------------------------
// SyncProductAttributes thực hiện sync thông minh:
// - Xóa giá trị bị bỏ chọn
// - Thêm giá trị mới
// - Giữ nguyên giá trị không thay đổi
func SyncProductAttributes(c *gin.Context, tx *sql.Tx, productID uint64, newAttrs map[uint32][]int) error {
	// 1. Lấy tất cả các bản ghi product_attribute hiện có trong DB của sản phẩm này
	rows, err := tx.QueryContext(c.Request.Context(),
		"SELECT id, attribute_id, attribute_value_id FROM product_attribute WHERE product_id = ?",
		productID,
	)
	if err != nil {
		return fmt.Errorf("lỗi đọc thuộc tính hiện tại từ DB: %v", err)
	}
	defer rows.Close()

	type existingRecord struct {
		ID               uint64
		AttributeID      uint32
		AttributeValueID int
	}

	// Map lưu các bản ghi hiện tại trong DB với Key = "AttrID_ValID"
	existingMap := make(map[string]uint64)
	for rows.Next() {
		var rec existingRecord
		if err := rows.Scan(&rec.ID, &rec.AttributeID, &rec.AttributeValueID); err != nil {
			return err
		}
		key := fmt.Sprintf("%d_%d", rec.AttributeID, rec.AttributeValueID)
		existingMap[key] = rec.ID
	}

	// 2. Chuyển Map từ FE gửi lên thành Set key "AttrID_ValID" để dễ so sánh
	incomingMap := make(map[string]bool)
	type pair struct {
		AttrID uint32
		ValID  int
	}
	var toInsert []pair

	for attrID, valIDs := range newAttrs {
		for _, valID := range valIDs {
			key := fmt.Sprintf("%d_%d", attrID, valID)
			incomingMap[key] = true

			// Nếu trong DB chưa có key này -> Cần thêm mới
			if _, exists := existingMap[key]; !exists {
				toInsert = append(toInsert, pair{AttrID: attrID, ValID: valID})
			}
		}
	}

	// 3. Lọc ra danh sách ID cần XÓA (có trong DB nhưng FE không gửi lên nữa)
	var idsToDelete []interface{}
	for key, dbID := range existingMap {
		if !incomingMap[key] {
			idsToDelete = append(idsToDelete, dbID)
		}
	}

	// 4. Thực hiện XÓA các bản ghi bị loại bỏ
	if len(idsToDelete) > 0 {
		deleteQuery := "DELETE FROM product_attribute WHERE id IN ("
		for i := range idsToDelete {
			if i > 0 {
				deleteQuery += ","
			}
			deleteQuery += "?"
		}
		deleteQuery += ")"

		_, err := tx.ExecContext(c.Request.Context(), deleteQuery, idsToDelete...)
		if err != nil {
			return fmt.Errorf("lỗi xóa thuộc tính thừa: %v", err)
		}
	}

	// 5. Thực hiện THÊM MỚI các bản ghi chưa có
	if len(toInsert) > 0 {
		insertQuery := "INSERT INTO product_attribute (product_id, attribute_id, attribute_value_id) VALUES "
		var insertVals []interface{}

		for i, p := range toInsert {
			if i > 0 {
				insertQuery += ","
			}
			insertQuery += "(?, ?, ?)"
			insertVals = append(insertVals, productID, p.AttrID, p.ValID)
		}

		_, err := tx.ExecContext(c.Request.Context(), insertQuery, insertVals...)
		if err != nil {
			return fmt.Errorf("lỗi thêm thuộc tính mới: %v", err)
		}
	}

	return nil
}
func deleteProductAttributesByIDs(c *gin.Context, tx *sql.Tx, productID uint64, ids []uint32) error {
	if len(ids) == 0 {
		return nil
	}
	query := `DELETE FROM product_attribute WHERE product_id = ? AND id IN (`
	args := []interface{}{productID}
	for i, id := range ids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"
	_, err := tx.ExecContext(c.Request.Context(), query, args...)
	return err
}

func deleteProductAttributesByAttrIDs(c *gin.Context, tx *sql.Tx, productID uint64, attrIDs []uint32) error {
	if len(attrIDs) == 0 {
		return nil
	}
	query := `DELETE FROM product_attribute WHERE product_id = ? AND attribute_id IN (`
	args := []interface{}{productID}
	for i, id := range attrIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"
	_, err := tx.ExecContext(c.Request.Context(), query, args...)
	return err
}

// SearchProductsService thực thi logic tìm kiếm, sắp xếp và phân trang sản phẩm
func SearchProductsService(c *gin.Context) (interface{}, int, error) {
	domainApi := c.Request.Host

	tenantId, exists := c.Get("tenantId")
	if !exists {
		return gin.H{"error": "Tenant context missing"}, http.StatusInternalServerError, fmt.Errorf("tenant context missing")
	}

	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	// 1. Lấy các tham số từ Query String
	searchText := c.Query("filter[q]")
	categoryID := c.Query("filter[category_id]")
	categorySlug := c.Query("filter[category_slug]")
	limit, _ := strconv.Atoi(c.DefaultQuery("filter[limit]", "20"))
	lang := c.DefaultQuery("lang", "en")

	sortBy := c.DefaultQuery("sort_by", "id")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	// 2. Query gốc
	query := `
		SELECT p.id, p.rating, p.offer_price, p.offer_start, p.offer_end, 
		       p.title, p.sale_price, p.slug, p.created_at, 
		       m.file_name, m.id as media_id, m.url
		FROM products as p 
		LEFT JOIN media as m ON m.id = (
			SELECT MAX(id) 
			FROM media 
			WHERE model_id = p.id 
			AND model_type = 'App\\Models\\Tenant\\Product' 
			AND collection_name = 'main_images'
		)
		WHERE 1=1`

	var args []interface{}
	whereClause := ""

	// Điều kiện tìm kiếm từ khóa
	if searchText != "" {
		likePattern := "%" + searchText + "%"
		whereClause += " AND (JSON_UNQUOTE(JSON_EXTRACT(p.title, ?)) LIKE ? OR JSON_UNQUOTE(JSON_EXTRACT(p.description, ?)) LIKE ?)"
		args = append(args, "$."+lang, likePattern, "$."+lang, likePattern)
	}

	// Lọc theo Category (Many-to-Many)
	if categoryID != "" || categorySlug != "" {
		whereClause += ` AND EXISTS (
			SELECT 1 
			FROM category_product cp
			INNER JOIN categories c_sub ON cp.category_id = c_sub.id
			WHERE cp.product_id = p.id`

		if categoryID != "" {
			whereClause += " AND cp.category_id = ?"
			args = append(args, categoryID)
		}

		if categorySlug != "" {
			whereClause += " AND c_sub.slug = ?"
			args = append(args, categorySlug)
		}

		whereClause += ")"
	}

	// Tính tổng số lượng bản ghi phục vụ phân trang
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM products as p WHERE 1 = 1 " + whereClause
	utils.LogSQL(countQuery, args...)

	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery, args...).Scan(&totalCount)
	if err != nil {
		log.Printf("Count error: %v", err)
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}

	query += whereClause

	// --- BẢO MẬT & SẮP XẾP ĐỘNG ---
	orderColumn := "p.id"
	orderDirection := "DESC"
	if strings.ToLower(sortOrder) == "asc" {
		orderDirection = "ASC"
	}

	switch sortBy {
	case "id":
		orderColumn = "p.id"
	case "title", "name":
		orderColumn = fmt.Sprintf("JSON_UNQUOTE(JSON_EXTRACT(p.title, '$.%s'))", lang)
	case "price", "sale_price":
		orderColumn = "p.sale_price"
	case "offer_price":
		orderColumn = "p.offer_price"
	case "stock", "stock_quantity":
		orderColumn = "p.stock_quantity"
	case "date", "created_at":
		orderColumn = "p.created_at"
	case "rating":
		orderColumn = "p.rating"
	default:
		orderColumn = "p.id"
	}

	query += fmt.Sprintf(" ORDER BY %s %s LIMIT ? OFFSET ?", orderColumn, orderDirection)

	mainArgs := append([]interface{}{}, args...)
	mainArgs = append(mainArgs, limit, offset)

	utils.LogSQL(query, mainArgs...)

	// 3. Thực thi Query Lấy Danh sách Sản phẩm
	rows, err := tenantDB.QueryContext(c.Request.Context(), query, mainArgs...)
	if err != nil {
		log.Printf("Search error: %v", err)
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	// 4. Parse kết quả và thu thập product_ids
	var products []map[string]interface{}
	var productIDs []uint64
	const timeLayout = "2006-01-02 15:04:05"

	for rows.Next() {
		p, err := utils.ScanRowToMap(rows)
		if err != nil {
			log.Println("err:", err)
			continue
		}

		pID := uint64(utils.ParseToFloat(p["id"]))
		productIDs = append(productIDs, pID)

		// Xử lý Image Link
		modelID := p["media_id"]
		fileName := p["file_name"]
		if modelID != nil && fileName != nil {
			p["image"] = fmt.Sprintf("http://%s/storage/tenancy/%s/app/public/%v/%v",
				domainApi, tenantId, modelID, fileName)
		} else {
			p["image"] = "https://tutaoweb.com/images/clothe.png"
		}
		if p["url"] != nil && p["url"] != "" {
			p["image"] = p["url"]
		}

		// Field mặc định
		p["link"] = fmt.Sprintf("http://%s/product/%v", domainApi, p["slug"])
		p["hasOption"] = false
		p["sub_images"] = []interface{}{}
		p["variants"] = []models.ProductVariantDetail{} // Khởi tạo mảng rỗng mặc định

		// Parse JSON cho title
		if titleStr, ok := p["title"].(string); ok {
			var titleObj interface{}
			if err := json.Unmarshal([]byte(titleStr), &titleObj); err == nil {
				p["name"] = titleObj
			}
		}

		// Tính toán giá và offer
		salePrice := utils.ParseToFloat(p["sale_price"])
		offerPrice := utils.ParseToFloat(p["offer_price"])
		now := time.Now()

		offerStart, _ := p["offer_start"].(time.Time)
		offerEnd, _ := p["offer_end"].(time.Time)

		hasOffer := false
		discountPercentage := 0.0

		if offerPrice > 0 && offerPrice < salePrice {
			isStarted := offerStart.IsZero() || now.After(offerStart) || now.Equal(offerStart)
			isNotExpired := offerEnd.IsZero() || now.Before(offerEnd) || now.Equal(offerEnd)

			if isStarted && isNotExpired {
				hasOffer = true
				discountPercentage = math.Round(((salePrice - offerPrice) / salePrice) * 100)
			}
		}

		if offerStartVal, ok := p["offer_start"].(time.Time); ok && !offerStartVal.IsZero() {
			p["offer_start"] = offerStartVal.Format(timeLayout)
		} else {
			p["offer_start"] = nil
		}

		if offerEndVal, ok := p["offer_end"].(time.Time); ok && !offerEndVal.IsZero() {
			p["offer_end"] = offerEndVal.Format(timeLayout)
		} else {
			p["offer_end"] = nil
		}

		if priceStr, ok := p["offer_price"].(string); ok {
			if priceVal, err := strconv.ParseFloat(priceStr, 64); err == nil {
				p["offer_price"] = fmt.Sprintf("%.2f", priceVal)
			}
		}

		p["has_offer"] = hasOffer
		p["discount_percentage"] = discountPercentage
		p["price"] = salePrice
		p["final_price"] = salePrice
		p["rate"] = utils.ParseToFloat(p["rating"])

		if hasOffer {
			p["final_price"] = offerPrice
		}

		products = append(products, p)
	}

	// 5. Query lấy danh sách VARIANTS theo batch (Eager Loading)
	if len(productIDs) > 0 {
		variantsMap, err := fetchVariantsForProducts(c, tenantDB, productIDs)
		if err == nil {
			for i, p := range products {
				pID := uint64(utils.ParseToFloat(p["id"]))
				if vars, exists := variantsMap[pID]; exists {
					products[i]["variants"] = vars
					if len(vars) > 0 {
						products[i]["hasOption"] = true
					}
				}
			}
		} else {
			log.Printf("Fetch variants error: %v", err)
		}
	}

	// Đóng gói theo cấu trúc Laravel Pagination
	response := utils.BuildLaravelPagination(c, products, totalCount, page, limit)
	return response, http.StatusOK, nil
}

// Helper query danh sách variants và option_value_ids tương ứng
func fetchVariantsForProducts(c *gin.Context, db *sqlx.DB, productIDs []uint64) (map[uint64][]models.ProductVariantDetail, error) {
	queryVars := `
		SELECT id, product_id, sku, price, compare_at_price, quantity, image, is_default
		FROM product_variants 
		WHERE product_id IN (?)`

	queryVars, argsVars, err := sqlx.In(queryVars, productIDs)
	if err != nil {
		return nil, err
	}
	queryVars = db.Rebind(queryVars)
	utils.LogSQL(queryVars, argsVars...)

	type VariantRow struct {
		ID             uint64   `db:"id"`
		ProductID      uint64   `db:"product_id"`
		SKU            string   `db:"sku"`
		Price          float64  `db:"price"`
		CompareAtPrice *float64 `db:"compare_at_price"`
		Quantity       int      `db:"quantity"`
		Image          *string  `db:"image"`
		IsDefault      bool     `db:"is_default"`
	}

	var varRows []VariantRow
	if err := db.SelectContext(c.Request.Context(), &varRows, queryVars, argsVars...); err != nil {
		return nil, err
	}

	if len(varRows) == 0 {
		return map[uint64][]models.ProductVariantDetail{}, nil
	}

	// Thu thập variantIDs để query option_values
	var variantIDs []uint64
	for _, v := range varRows {
		variantIDs = append(variantIDs, v.ID)
	}

	// Query mapping Option Values cho từng Variant
	optionsMap := make(map[uint64][]uint64)
	if len(variantIDs) > 0 {
		qOpt := `SELECT variant_id, option_value_id FROM product_variant_option_values WHERE variant_id IN (?)`
		qOpt, argsOpt, err := sqlx.In(qOpt, variantIDs)
		if err == nil {
			qOpt = db.Rebind(qOpt)
			utils.LogSQL(qOpt, argsOpt...)

			type OptRow struct {
				VariantID     uint64 `db:"variant_id"`
				OptionValueID uint64 `db:"option_value_id"`
			}
			var optRows []OptRow
			if err := db.SelectContext(c.Request.Context(), &optRows, qOpt, argsOpt...); err == nil {
				for _, opt := range optRows {
					optionsMap[opt.VariantID] = append(optionsMap[opt.VariantID], opt.OptionValueID)
				}
			}
		}
	}

	// Group variants vào map theo ProductID
	result := make(map[uint64][]models.ProductVariantDetail)
	for _, v := range varRows {
		img := ""
		if v.Image != nil {
			img = *v.Image
		}

		optIDs := optionsMap[v.ID]
		if optIDs == nil {
			optIDs = []uint64{}
		}

		item := models.ProductVariantDetail{
			ID:             v.ID,
			SKU:            v.SKU,
			Price:          v.Price,
			CompareAtPrice: v.CompareAtPrice,
			Quantity:       v.Quantity,
			Image:          img,
			IsDefault:      v.IsDefault,
			OptionValueIDs: optIDs,
		}

		result[v.ProductID] = append(result[v.ProductID], item)
	}

	return result, nil
}

func SyncProductVariants(c *gin.Context, tx *sql.Tx, productID uint64, incomingVariants []models.VariantInput) error {
	// 1. Lấy danh sách Variant ID hiện có trong DB
	rows, err := tx.QueryContext(c.Request.Context(), "SELECT id FROM product_variants WHERE product_id = ?", productID)
	if err != nil {
		return fmt.Errorf("lỗi truy vấn variants cũ: %v", err)
	}
	defer rows.Close()

	existingIDs := make(map[uint64]bool)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err == nil {
			existingIDs[id] = true
		}
	}

	incomingIDs := make(map[uint64]bool)

	// 2. Lặp qua các Variant FE gửi lên -> Cập nhật (Update) hoặc Thêm mới (Insert)
	for _, v := range incomingVariants {
		var variantID uint64

		if v.ID > 0 && existingIDs[v.ID] {
			// UPDATE Variant cũ
			variantID = v.ID
			incomingIDs[variantID] = true

			updateQuery := `UPDATE product_variants SET 
								sku = ?, price = ?, compare_at_price = ?, quantity = ?, image = ?, is_default = ?, updated_at = NOW() 
							WHERE id = ? AND product_id = ?`
			_, err := tx.ExecContext(c.Request.Context(), updateQuery, v.SKU, v.Price, v.CompareAtPrice, v.Quantity, v.Image, v.IsDefault, variantID, productID)
			if err != nil {
				return fmt.Errorf("lỗi cập nhật variant %d: %v", variantID, err)
			}
		} else {
			// INSERT Variant mới
			insertQuery := `INSERT INTO product_variants (product_id, sku, price, compare_at_price, quantity, image, is_default, created_at, updated_at) 
							VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`
			res, err := tx.ExecContext(c.Request.Context(), insertQuery, productID, v.SKU, v.Price, v.CompareAtPrice, v.Quantity, v.Image, v.IsDefault)
			if err != nil {
				return fmt.Errorf("lỗi thêm mới variant: %v", err)
			}
			newID, _ := res.LastInsertId()
			variantID = uint64(newID)
		}

		// 3. Cập nhật bảng liên kết product_variant_option_values
		// Xóa liên kết option_value cũ của Variant này
		_, _ = tx.ExecContext(c.Request.Context(), "DELETE FROM product_variant_option_values WHERE variant_id = ?", variantID)

		if len(v.OptionValueIDs) > 0 {
			pvovQuery := `INSERT INTO product_variant_option_values (variant_id, option_id, option_value_id) 
						  SELECT ?, option_id, id FROM option_values WHERE id IN (`

			var args []interface{}
			args = append(args, variantID)
			for i, ovID := range v.OptionValueIDs {
				if i > 0 {
					pvovQuery += ","
				}
				pvovQuery += "?"
				args = append(args, ovID)
			}
			pvovQuery += ")"

			_, err = tx.ExecContext(c.Request.Context(), pvovQuery, args...)
			if err != nil {
				return fmt.Errorf("lỗi liên kết option_value cho variant %d: %v", variantID, err)
			}
		}
	}

	// 4. Xóa các Variant cũ không còn nằm trong payload gửi lên
	var idsToDelete []interface{}
	for id := range existingIDs {
		if !incomingIDs[id] {
			idsToDelete = append(idsToDelete, id)
		}
	}

	if len(idsToDelete) > 0 {
		deleteQuery := "DELETE FROM product_variants WHERE id IN ("
		for i := range idsToDelete {
			if i > 0 {
				deleteQuery += ","
			}
			deleteQuery += "?"
		}
		deleteQuery += ")"
		_, err = tx.ExecContext(c.Request.Context(), deleteQuery, idsToDelete...)
		if err != nil {
			return fmt.Errorf("lỗi xóa variant thừa: %v", err)
		}
	}

	return nil
}
