package controllers

import (
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

func GetAttributes(c *gin.Context) {
	data, statusCode, err := services.GetAttributesService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func GetAttributeByID(c *gin.Context) {
	data, statusCode, err := services.GetAttributeByIDService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func CreateAttribute(c *gin.Context) {
	data, statusCode, err := services.CreateAttributeService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func UpdateAttribute(c *gin.Context) {
	data, statusCode, err := services.UpdateAttributeService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func DeleteAttribute(c *gin.Context) {
	data, statusCode, err := services.DeleteAttributeService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}
