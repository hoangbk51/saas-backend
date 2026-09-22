package controllers

import (
	"go-saas/services" // Thay bằng path thực tế của bạn
	"net/http"
	"strconv"

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

func ListBlogCategories(c *gin.Context) {
	list, err := services.ListBlogCategoriesService(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func GetBlogCategoryDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.GetBlogCategoryDetailService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func CreateBlogCategory(c *gin.Context) {
	res, code, err := services.CreateBlogCategoryService(c)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func UpdateBlogCategory(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.UpdateBlogCategoryService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func DeleteBlogCategory(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.DeleteBlogCategoryService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}
