package services

import (
	"go-saas/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type ContactInput struct {
	Name    string `json:"name" binding:"required"`
	Email   string `json:"email" binding:"required,email"`
	Phone   string `json:"phone"`
	Subject string `json:"subject"`
	Message string `json:"message" binding:"required"`
}

func SaveContact(c *gin.Context, input ContactInput) (int64, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, err
	}

	query := `
		INSERT INTO contacts (name, email, phone, subject, message, created_at) 
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	args := []interface{}{input.Name, input.Email, input.Phone, input.Subject, input.Message, now}

	// Ghi log SQL để debug bằng hàm bạn vừa tạo trong utils
	utils.LogSQL(query, args...)

	result, err := db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	lastID, _ := result.LastInsertId()
	return lastID, nil
}
