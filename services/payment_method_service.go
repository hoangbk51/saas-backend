package services

import (
	"database/sql"
	"fmt"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type PaymentMethod struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func GetAllPayments(c *gin.Context) ([]PaymentMethod, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id,code, JSON_UNQUOTE(JSON_EXTRACT(name, '$.en')) as name FROM payment_methods ORDER BY name ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []PaymentMethod
	for rows.Next() {
		var c PaymentMethod
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		payments = append(payments, c)
	}
	return payments, nil
}

func GetPaymentMethodById(c *gin.Context, id int64) (*PaymentMethod, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	p := &PaymentMethod{}

	// Query lấy chi tiết và Join lấy tên Parent
	query := `
        SELECT 
           id,code,JSON_UNQUOTE(JSON_EXTRACT(name, '$.en')) as name  
        FROM payment_methods 
        WHERE id = ? 
        LIMIT 1
    `
	utils.LogSQL(query, id)

	err = db.QueryRow(query, id).Scan(
		&p.ID,
		&p.Code,
		&p.Name,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Payment Method không tồn tại")
		}
		return nil, err
	}

	return p, nil
}

func GetPaymentMethodByCode(c *gin.Context, code string) (*PaymentMethod, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	p := &PaymentMethod{}

	// Query lấy chi tiết và Join lấy tên Parent
	query := `
        SELECT 
           id,code,JSON_UNQUOTE(JSON_EXTRACT(name, '$.en')) as name  
        FROM payment_methods 
        WHERE code = ? 
        LIMIT 1
    `
	utils.LogSQL(query, code)

	err = db.QueryRow(query, code).Scan(
		&p.ID,
		&p.Code,
		&p.Name,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Payment Method không tồn tại")
		}
		return nil, err
	}

	return p, nil
}
