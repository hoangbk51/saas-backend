package services

import (
	"database/sql"
	"go-saas/utils"

	"go-saas/models"

	"github.com/gin-gonic/gin"
)

func GetAllProductItemTemplates(c *gin.Context) ([]models.ProductItemTemplate, error) {
	db, err := utils.GetDBFromContext(c)

	query := `SELECT id,  data,  created_at FROM product_item_templates WHERE deleted_at IS NULL ORDER BY id DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.ProductItemTemplate
	for rows.Next() {
		var h models.ProductItemTemplate
		rows.Scan(&h.ID, &h.Data, &h.CreatedAt)
		histories = append(histories, h)
	}
	return histories, nil
}

func CreateProductItemTemplate(c *gin.Context, h *models.ProductItemTemplate) (int64, error) {
	db, err := utils.GetDBFromContext(c)

	query := `INSERT INTO product_item_templates ( data,created_at, updated_at) VALUES (?,   NOW(), NOW())`
	// Convert RawMessage ([]byte) sang string để lưu vào longtext
	args := []interface{}{string(h.Data)}
	utils.LogSQL(query, args...)
	res, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateProductItemTemplate(c *gin.Context, id string, h *models.ProductItemTemplate) error {
	db, err := utils.GetDBFromContext(c)

	// Kiểm tra sự tồn tại trước khi update (optional nhưng nên có)
	query := `UPDATE product_item_templates 
              SET  data = ?,  updated_at = NOW() 
              WHERE id = ? AND deleted_at IS NULL`

	res, err := db.Exec(query, h.Data, id)
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows // Hoặc một lỗi tùy chỉnh nếu ID không tồn tại
	}

	return nil
}

func GetProductItemTemplateByID(c *gin.Context, id string) (*models.ProductItemTemplate, error) {
	db, err := utils.GetDBFromContext(c)

	var h models.ProductItemTemplate
	query := `SELECT id,  data FROM product_item_templates WHERE id = ? AND deleted_at IS NULL`
	err = db.QueryRow(query, id).Scan(&h.ID, &h.Data)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func DeleteProductItemTemplate(c *gin.Context, id string) error {
	db, err := utils.GetDBFromContext(c)

	// Soft delete theo style Laravel
	query := `UPDATE product_item_templates SET deleted_at = NOW() WHERE id = ?`
	_, err = db.Exec(query, id)
	return err
}

func GetProductItemTemplateData(c *gin.Context) ([]models.ProductItemTemplate, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	// SỬA TẠI ĐÂY: Thêm id và created_at vào câu SELECT
	query := `SELECT data FROM product_item_templates 
              WHERE deleted_at IS NULL 
              ORDER BY created_at DESC LIMIT 30`

	utils.LogSQL(query)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var productItens []models.ProductItemTemplate
	for rows.Next() {
		var h models.ProductItemTemplate

		// Scan bây giờ sẽ khớp với 3 cột: id, data, created_at
		err := rows.Scan(&h.Data)
		if err != nil {
			return nil, err // Quan trọng: Nên check lỗi ở đây
		}
		productItens = append(productItens, h)
	}

	return productItens, nil
}
