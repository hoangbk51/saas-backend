package services

import (
	"go-saas/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type NewsletterInput struct {
	Email string `json:"email" binding:"required,email"`
}

func SaveNewsletter(c *gin.Context, input NewsletterInput) (int64, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, err
	}

	query := `
		INSERT INTO newsletters ( email, created_at) 
		VALUES (?, ?)
	`
	now := time.Now()
	args := []interface{}{input.Email, now}

	// Ghi log SQL để debug bằng hàm bạn vừa tạo trong utils
	utils.LogSQL(query, args...)

	result, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	lastID, _ := result.LastInsertId()
	return lastID, nil
}
