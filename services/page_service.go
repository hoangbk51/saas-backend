package services

import (
	"database/sql"
	"fmt"
	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func GetPages(c *gin.Context) ([]models.Page, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := `SELECT id, author_id, title, slug, content, published_at, visibility, position, created_at 
              FROM pages WHERE deleted_at IS NULL ORDER BY id DESC`

	rows, err := db.Query(query) // Dùng QueryContext để hỗ trợ timeout/cancel
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []models.Page
	for rows.Next() {
		var p models.Page
		err := rows.Scan(&p.ID, &p.AuthorID, &p.Title, &p.Slug, &p.Content, &p.PublishedAt, &p.Visibility, &p.Position, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, nil
}

func CreatePage(c *gin.Context, p *models.Page) (int64, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, err
	}

	query := `INSERT INTO pages (author_id, title, slug, content, published_at, visibility, position, created_at, updated_at) 
              VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`
	args := []interface{}{p.AuthorID, p.Title, p.Slug, p.Content, p.PublishedAt, p.Visibility, p.Position}
	utils.LogSQL(query, args...)
	res, err := db.Exec(query, args...)
	pageID, err := res.LastInsertId()
	if err != nil {
		// Xử lý lỗi nếu không lấy được ID
		return 0, err
	}
	_, err = HandleFileUpload(c, UploadParam{
		ModelType:      "App\\Models\\Tenant\\Page",
		ModelID:        pageID,
		CollectionName: "main_images",
		FieldName:      "image",
	})

	if err != nil {
		// Log lỗi nhưng có thể vẫn trả về success cho product nếu ảnh không bắt buộc
	}

	return pageID, err
}

// Cập nhật trang
func UpdatePage(c *gin.Context, id string, p *models.Page) error {
	db, err := utils.GetDBFromContext(c)

	query := `UPDATE pages SET author_id = ?, title = ?, slug = ?, content = ?, visibility = ?, position = ?, updated_at = NOW() 
              WHERE id = ? AND deleted_at IS NULL`

	_, err = db.Exec(query, p.AuthorID, p.Title, p.Slug, p.Content, p.Visibility, p.Position, id)
	return err
}

// Xóa mềm (Soft Delete)
func DeletePage(c *gin.Context, id string) error {
	db, err := utils.GetDBFromContext(c)

	query := `UPDATE pages SET deleted_at = NOW() WHERE id = ?`
	_, err = db.Exec(query, id)
	return err
}

func GetPageDetailById(c *gin.Context, identifier string) (*models.Page, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	p := &models.Page{}
	var publishedAt sql.NullTime

	// Query xử lý COALESCE để tránh lỗi Scan NULL
	query := `
        SELECT 
            id, 
            COALESCE(author_id, 0), 
			JSON_UNQUOTE(JSON_EXTRACT(title, '$.en')) , 
            COALESCE(slug, ''), 
			JSON_UNQUOTE(JSON_EXTRACT(content, '$.en')) , 
            published_at, 
            visibility, 
            COALESCE(position, ''), 
            created_at, 
            updated_at
        FROM pages
        WHERE (id = ? ) AND deleted_at IS NULL
        LIMIT 1
    `

	err = db.QueryRow(query, identifier).Scan(
		&p.ID,
		&p.AuthorID,
		&p.Title,
		&p.Slug,
		&p.Content,
		&publishedAt,
		&p.Visibility,
		&p.Position,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("trang không tồn tại")
		}
		return nil, err
	}

	// Xử lý chuyển đổi sql.NullTime sang con trỏ *time.Time cho đẹp JSON
	if publishedAt.Valid {
		p.PublishedAt = &publishedAt.Time
	}

	return p, nil
}
