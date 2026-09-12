package controllers

import (
	"database/sql"
	"go-saas/models"
	"go-saas/services"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetProductItemTemplates(c *gin.Context) {
	data, err := services.GetAllPageHistories(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, data)
}

func StoreProductItemTemplate(c *gin.Context) {
	var input models.ProductItemTemplate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	id, err := services.CreateProductItemTemplate(c, &input)
	if err != nil {
		c.JSON(500, gin.H{"error": "Không thể lưu lịch sử trang"})
		return
	}
	c.JSON(200, gin.H{"id": id, "message": "Đã lưu giao dien product item"})
}

func UpdateProductItemTemplate(c *gin.Context) {
	id := c.Param("id")
	var input models.ProductItemTemplate

	// Parse JSON body
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	// Thực hiện update
	err := services.UpdateProductItemTemplate(c, id, &input)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy bản ghi để cập nhật"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống khi cập nhật"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cập nhật lịch sử trang thành công",
		"id":      id,
	})
}

func ShowProductItemTemplate(c *gin.Context) {
	id := c.Param("id")
	history, err := services.GetProductItemTemplateByID(c, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "Không tìm thấy bản ghi"})
		return
	}
	c.JSON(200, history)
}

func DestroyProductItemTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeleteProductItemTemplate(c, id); err != nil {
		c.JSON(500, gin.H{"error": "Xóa thất bại"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã xóa bản ghi"})
}

func GetProductItemTemplateData(c *gin.Context) {

	data, err := services.GetProductItemTemplateData(c)
	if err != nil {
		// In ra console/file log để debug
		log.Printf("[Critical] Service Error: %v", err)

		// Trả về lỗi chi tiết cho client (nếu đang ở môi trường dev)
		c.JSON(500, gin.H{"error": "Lỗi truy vấn dữ liệu", "detail": err.Error()})
		return
	}

	if data == nil {
		c.JSON(404, gin.H{"message": "Chưa có bản ghi lịch sử nào"})
		return
	}

	c.JSON(200, gin.H{
		"data": data,
	})
}
