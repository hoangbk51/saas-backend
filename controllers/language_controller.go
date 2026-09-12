package controllers

import (
	"go-saas/services" // Thay bằng path thực tế của bạn

	"github.com/gin-gonic/gin"
)

func AllLanguages(c *gin.Context) {

	languages, err := services.GetAllLanguages(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": languages})
}

func FrontendLanguages(c *gin.Context) {

	languages, err := services.FrontendLanguages(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": languages})
}
