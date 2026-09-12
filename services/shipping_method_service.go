package services

import (
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type ShippingMethod struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func GetAllShippingMethods(c *gin.Context) ([]ShippingMethod, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id,code, name FROM shipping_methods ORDER BY name ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shippingmethods []ShippingMethod
	for rows.Next() {
		var c ShippingMethod
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		shippingmethods = append(shippingmethods, c)
	}
	return shippingmethods, nil
}
