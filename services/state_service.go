package services

import (
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type State struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func StatesByCountry(c *gin.Context, countryID string) ([]State, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetCentralDB()

	if err != nil {
		return nil, err
	}

	query := "SELECT id, name FROM states where country_id = ? ORDER BY name ASC"
	rows, err := db.Query(query, countryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []State
	for rows.Next() {
		var c State
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			continue
		}
		states = append(states, c)
	}
	return states, nil
}
