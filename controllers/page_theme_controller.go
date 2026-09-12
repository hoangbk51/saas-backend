package controllers

import (
	"go-saas/services"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetPageThemesHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	list, total, err := services.GetAllPageThemes(c, offset, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"data": list,
		"meta": gin.H{"total": total, "page": page, "limit": limit},
	})
}

func GetOtherPageThemesHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	list, total, err := services.GetOtherPageThemes(c, offset, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"data": list,
		"meta": gin.H{"total": total, "page": page, "limit": limit},
	})
}

func CreatePageThemeHandler(c *gin.Context) {
	var input services.PageTheme
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	id, err := services.CreatePageTheme(c, input)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Created", "id": id})
}

func UpdatePageThemeHandler(c *gin.Context) {
	id := c.Param("id")
	var input services.PageTheme
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := services.UpdatePageTheme(c, id, input); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Updated"})
}

func DeletePageThemeHandler(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeletePageTheme(c, id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Deleted"})
}
