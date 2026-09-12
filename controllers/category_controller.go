package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func ListCategories(c *gin.Context) {
	cats, err := services.GetCategories(c)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "success", "data": cats})
}

func StoreCategory(c *gin.Context) {
	var input models.CategoryPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"status": "error", "message": "Dữ liệu không hợp lệ"})
		return
	}

	if err := services.CreateCategory(c, input); err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "success", "message": "Thêm danh mục thành công"})
}

func DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	db, _ := utils.GetDBFromContext(c)

	// Soft Delete giống Laravel
	_, err := db.Exec("UPDATE categories SET deleted_at = NOW() WHERE id = ?", id)
	if err != nil {
		c.JSON(500, gin.H{"status": "error", "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "success", "message": "Đã xóa danh mục"})
}

func CategoryDetail(c *gin.Context) {
	// Lấy ID từ URL param: /categories/:id
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID không hợp lệ"})
		return
	}

	category, err := services.GetCategoryDetail(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   category,
	})
}

func CategoryTree(c *gin.Context) {
	tree, err := services.GetCategoryTree(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   tree,
	})

}
