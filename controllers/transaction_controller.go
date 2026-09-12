package controllers

import (
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

func GetCustomerTransactions(c *gin.Context) {
	response, statusCode, err := services.GetCustomerTransactions(c)
	if err != nil {
		c.JSON(statusCode, response)
		return
	}

	c.JSON(statusCode, response)
}
