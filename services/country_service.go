package services

import (
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type Country struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func GetAllCountries(c *gin.Context) ([]Country, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetCentralDB()
	if err != nil {
		return nil, err
	}

	query := "SELECT id, iso_3166_2, name FROM countries ORDER BY name ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var countries []Country
	for rows.Next() {
		var c Country
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		countries = append(countries, c)
	}
	return countries, nil
}
