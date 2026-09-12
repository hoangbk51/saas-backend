package controllers

import (
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AllCurrencies(c *gin.Context) {
	// Gọi thẳng hàm từ service (Cách 1 - không tiêm struct)
	list, err := services.GetAllCurrencies(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   list,
	})
}
