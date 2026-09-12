package services

import (
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type Currency struct {
	ID     int     `json:"id"`
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Symbol string  `json:"symbol"`
	Rate   float64 `json:"exchange_rate"`
}

// GetAllCurrencies lấy toàn bộ danh sách tiền tệ
func GetAllCurrencies(c *gin.Context) ([]Currency, error) {
	db, err := utils.GetCentralDB()
	if err != nil {
		return nil, err
	}

	query := "SELECT id, iso_code as code, name, symbol, exchange_rate FROM currencies where active = 1 ORDER BY code ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Currency
	for rows.Next() {
		var cur Currency
		if err := rows.Scan(&cur.ID, &cur.Code, &cur.Name, &cur.Symbol, &cur.Rate); err != nil {
			continue
		}
		list = append(list, cur)
	}

	return list, nil
}
