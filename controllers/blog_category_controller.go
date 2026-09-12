package controllers

import (
	"go-saas/services" // Thay bằng path thực tế của bạn

	"github.com/gin-gonic/gin"
)

func AllBlogCategories(c *gin.Context) {

	blogCategories, err := services.GetAllBlogCategories(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": blogCategories})
}
