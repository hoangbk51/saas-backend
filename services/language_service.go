package services

import (
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type Language struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func GetAllLanguages(c *gin.Context) ([]Language, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id,code, language as name FROM languages ORDER BY name ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var languages []Language
	for rows.Next() {
		var c Language
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		languages = append(languages, c)
	}
	return languages, nil
}

func FrontendLanguages(c *gin.Context) ([]Language, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id,code, language as name FROM languages where active = 1 and is_frontend = 1 ORDER BY name ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var languages []Language
	for rows.Next() {
		var c Language
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		languages = append(languages, c)
	}
	return languages, nil
}
