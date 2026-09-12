package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListPages(c *gin.Context) {
	pages, err := services.GetPages(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi truy vấn"})
		return
	}
	c.JSON(http.StatusOK, pages)
}

func StorePage(c *gin.Context) {
	var input models.Page
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := services.CreatePage(c, &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu trang"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "Trang đã được tạo"})
}

func UpdatePage(c *gin.Context) {
	id := c.Param("id")
	var input models.Page
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := services.UpdatePage(c, id, &input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cập nhật thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cập nhật thành công"})
}

func DestroyPage(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeletePage(c, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Xóa thất bại"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã xóa trang"})
}

func GetPageById(c *gin.Context) {
	// identifier có thể là ID hoặc Slug từ URL
	pageId := c.Param("id")

	page, err := services.GetPageDetailById(c, pageId)
	if err != nil {
		c.JSON(404, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status": "success",
		"data":   page,
	})
}
