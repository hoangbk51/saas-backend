package controllers

import (
	"go-saas/services" // Thay bằng path thực tế của bạn

	"github.com/gin-gonic/gin"
)

func GetCountries(c *gin.Context) {
	// Không cần lấy db ở đây nữa!

	countries, err := services.GetAllCountries(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": countries})
}
