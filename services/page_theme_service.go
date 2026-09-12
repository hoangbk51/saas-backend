package services

import (
	"go-saas/utils" // Thay bằng module path của bạn
	"time"

	"github.com/gin-gonic/gin"
)

type PageTheme struct {
	ID              int        `json:"id"`
	ThemeID         int        `json:"theme_id"`
	PageType        int        `json:"page_type"`
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Name            string     `json:"name"`
	MetaDescription string     `json:"meta_description"`
	MetaKeywords    string     `json:"meta_keywords"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at"` // Dùng con trỏ cho giá trị NULL
}

// 1. Get All (Có Phân Trang)
func GetAllPageThemes(c *gin.Context, offset, limit int) ([]PageTheme, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, 0, err
	}

	var total int
	db.QueryRow("SELECT COUNT(*) FROM page_themes WHERE deleted_at IS NULL").Scan(&total)

	query := "SELECT id, theme_id, page_type, slug, title, meta_description, meta_keywords, created_at FROM page_themes WHERE deleted_at IS NULL LIMIT ? OFFSET ?"
	utils.LogSQL(query, limit, offset)

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []PageTheme
	for rows.Next() {
		var p PageTheme
		rows.Scan(&p.ID, &p.ThemeID, &p.PageType, &p.Slug, &p.Title, &p.MetaDescription, &p.MetaKeywords, &p.CreatedAt)
		list = append(list, p)
	}
	return list, total, nil
}

func GetOtherPageThemes(c *gin.Context, offset, limit int) ([]PageTheme, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, 0, err
	}

	var total int
	db.QueryRow("SELECT COUNT(*) FROM page_themes WHERE page_type = 23 and  deleted_at IS NULL").Scan(&total)

	query := "SELECT id, theme_id, page_type, slug, title,title as name, meta_description, meta_keywords, created_at FROM page_themes WHERE  page_type = 23 and deleted_at IS NULL LIMIT ? OFFSET ?"
	utils.LogSQL(query, limit, offset)

	rows, err := db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []PageTheme
	for rows.Next() {
		var p PageTheme
		rows.Scan(&p.ID, &p.ThemeID, &p.PageType, &p.Slug, &p.Title, &p.Name, &p.MetaDescription, &p.MetaKeywords, &p.CreatedAt)
		list = append(list, p)
	}
	return list, total, nil
}

// 2. Create
func CreatePageTheme(c *gin.Context, p PageTheme) (int64, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, err
	}

	query := `INSERT INTO page_themes (theme_id, page_type, slug, title, meta_description, meta_keywords, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`
	args := []interface{}{p.ThemeID, p.PageType, p.Slug, p.Title, p.MetaDescription, p.MetaKeywords}

	utils.LogSQL(query, args...)
	res, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// 3. Update
func UpdatePageTheme(c *gin.Context, id string, p PageTheme) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	query := `UPDATE page_themes SET theme_id=?, page_type=?, slug=?, title=?, meta_description=?, meta_keywords=?, updated_at=NOW() 
			  WHERE id=? AND deleted_at IS NULL`
	args := []interface{}{p.ThemeID, p.PageType, p.Slug, p.Title, p.MetaDescription, p.MetaKeywords, id}

	utils.LogSQL(query, args...)
	_, err = db.Exec(query, args...)
	return err
}

// 4. Delete (Soft Delete)
func DeletePageTheme(c *gin.Context, id string) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	query := "UPDATE page_themes SET deleted_at = NOW() WHERE id = ?"
	utils.LogSQL(query, id)
	_, err = db.Exec(query, id)
	return err
}
