package services

import (
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type BlogCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func GetAllBlogCategories(c *gin.Context) ([]BlogCategory, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id, name FROM blog_categories ORDER BY id ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blogCategories []BlogCategory
	for rows.Next() {
		var c BlogCategory
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			continue
		}
		blogCategories = append(blogCategories, c)
	}
	return blogCategories, nil
}
