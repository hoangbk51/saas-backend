package controllers

import (
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

func GetOptions(c *gin.Context) {
	resp, code, err := services.GetOptionsService(c)
	if err != nil && resp == nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, resp)
}

func GetOptionByID(c *gin.Context) {
	resp, code, _ := services.GetOptionByIDService(c)
	c.JSON(code, resp)
}

func CreateOption(c *gin.Context) {
	resp, code, _ := services.CreateOptionService(c)
	c.JSON(code, resp)
}

func UpdateOption(c *gin.Context) {
	resp, code, _ := services.UpdateOptionService(c)
	c.JSON(code, resp)
}

func DeleteOption(c *gin.Context) {
	resp, code, _ := services.DeleteOptionService(c)
	c.JSON(code, resp)
}
