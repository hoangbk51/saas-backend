package services

import (
	"database/sql"
	"encoding/json"
	"go-saas/utils"

	"go-saas/models"

	"github.com/gin-gonic/gin"
)

func GetAllPageHistories(c *gin.Context) ([]models.PageHistory, error) {
	db, err := utils.GetDBFromContext(c)

	query := `SELECT id,  data, page_theme_id, created_at FROM page_histories WHERE deleted_at IS NULL ORDER BY id DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.PageHistory
	for rows.Next() {
		var h models.PageHistory
		rows.Scan(&h.ID, &h.Data, &h.PageThemeID, &h.CreatedAt)
		histories = append(histories, h)
	}
	return histories, nil
}

func CreatePageHistory(c *gin.Context, h *models.PageHistory) error {
	db, err := utils.GetDBFromContext(c)

	query := `INSERT INTO page_histories ( data, page_theme_id, created_at, updated_at) VALUES (?, ?,  NOW(), NOW())`
	// Convert RawMessage ([]byte) sang string để lưu vào longtext
	args := []interface{}{string(h.Data), h.PageThemeID}
	utils.LogSQL(query, args...)
	_, err = db.Exec(query, args...)
	if err != nil {
		return err
	}

	query = `INSERT INTO page_histories ( data, page_theme_id, created_at, updated_at) VALUES (?, 1, NOW(), NOW())`
	// Convert RawMessage ([]byte) sang string để lưu vào longtext
	args = []interface{}{string(h.Header)}
	utils.LogSQL(query, args...)
	_, err = db.Exec(query, args...)
	if err != nil {
		return err
	}

	query = `INSERT INTO page_histories ( data, page_theme_id, created_at, updated_at) VALUES (?, 2,  NOW(), NOW())`
	// Convert RawMessage ([]byte) sang string để lưu vào longtext
	args = []interface{}{string(h.Footer)}
	utils.LogSQL(query, args...)
	_, err = db.Exec(query, args...)
	if err != nil {
		return err
	}

	return err
}

func UpdatePageHistory(c *gin.Context, id string, h *models.PageHistory) error {
	db, err := utils.GetDBFromContext(c)

	// Kiểm tra sự tồn tại trước khi update (optional nhưng nên có)
	query := `UPDATE page_histories 
              SET  data = ?, page_theme_id = ?, updated_at = NOW() 
              WHERE id = ? AND deleted_at IS NULL`

	res, err := db.Exec(query, h.Data, h.PageThemeID, id)
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return sql.ErrNoRows // Hoặc một lỗi tùy chỉnh nếu ID không tồn tại
	}

	return nil
}

func GetPageHistoryByID(c *gin.Context, id string) (*models.PageHistory, error) {
	db, err := utils.GetDBFromContext(c)

	var h models.PageHistory
	query := `SELECT id,  data, page_theme_id FROM page_histories WHERE id = ? AND deleted_at IS NULL`
	err = db.QueryRow(query, id).Scan(&h.ID, &h.Data, &h.PageThemeID)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func DeletePageHistory(c *gin.Context, id string) error {
	db, err := utils.GetDBFromContext(c)

	// Soft delete theo style Laravel
	query := `UPDATE page_histories SET deleted_at = NOW() WHERE id = ?`
	_, err = db.Exec(query, id)
	return err
}

func GetLatestPageData(c *gin.Context, pageThemeID int64) (json.RawMessage, error) {
	db, err := utils.GetDBFromContext(c)

	var dataStr string

	// Query lấy bản ghi mới nhất (DESC + LIMIT 1)
	// Nếu bạn thực sự muốn bản ghi cũ nhất thì đổi thành ASC
	query := `SELECT data FROM page_histories 
              WHERE page_theme_id = ? AND deleted_at IS NULL 
              ORDER BY created_at DESC LIMIT 1`

	utils.LogSQL(query, pageThemeID)

	err = db.QueryRow(query, pageThemeID).Scan(&dataStr)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Không có dữ liệu
		}
		return nil, err
	}

	// Chuyển string từ DB thành RawMessage (Tương đương json_decode)
	return json.RawMessage(dataStr), nil
}
