package controllers

import (
	"fmt"
	"go-saas/models"
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

func LogAIUsage(c *gin.Context) {
	var payload models.AIUsage
	if err := c.ShouldBindJSON(&payload); err != nil {
		//c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		fmt.Printf("Binding Error: %v\n", err)

		c.JSON(400, gin.H{
			"error":   "Dữ liệu không hợp lệ",
			"details": err.Error(), // Trả về chi tiết để debug cho nhanh
		})
		return
	}

	if err := services.LogAIUsage(c, &payload); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Lưu usage AI thành công"})
}
