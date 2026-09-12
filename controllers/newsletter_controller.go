package controllers

import (
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SaveNewsletter(c *gin.Context) {
	var input services.NewsletterInput

	// Validate JSON gửi lên
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Dữ liệu không hợp lệ",
			"details": err.Error(),
		})
		return
	}

	// Gọi service để lưu
	id, err := services.SaveNewsletter(c, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Không thể lưu thông tin liên hệ",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Cảm ơn bạn đã liên hệ!",
		"id":      id,
	})
}
