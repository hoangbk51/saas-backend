package services

import (
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type ReviewInput struct {
	ProductID  int    `json:"product_id" binding:"required"`
	Comment    string `json:"comment" binding:"required"`
	Rating     int    `json:"rating" binding:"required,min=1,max=5"`
	CustomerID int    `json:"customer_id"`
}

func CreateReview(c *gin.Context, input ReviewInput) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	query := `INSERT INTO reviews (product_id, comment, customer_id, rating, created_at) 
			  VALUES (?, ?, ?, ?, NOW())`

	args := []interface{}{input.ProductID, input.Comment, input.CustomerID, input.Rating}

	// Ghi log SQL để debug
	utils.LogSQL(query, args...)

	_, err = db.Exec(query, args...)
	return err
}
