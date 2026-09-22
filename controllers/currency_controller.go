package controllers

import (
	"go-saas/services"
	"net/http"
	"strconv"

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

func GetCurrencyDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.GetCurrencyDetailService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func CreateCurrency(c *gin.Context) {
	res, code, err := services.CreateCurrencyService(c)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func UpdateCurrency(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.UpdateCurrencyService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func DeleteCurrency(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.DeleteCurrencyService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}
