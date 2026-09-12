package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func ProductSearch(c *gin.Context) {
	responseData, statusCode, _ := services.SearchProductsService(c)
	c.JSON(statusCode, responseData)
}

func CreateProduct(c *gin.Context) {
	// ShouldBind tự động nhận diện Content-Type (JSON hoặc Multipart Form)
	var req models.ProductSaveRequest

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(400, gin.H{"error": "Dữ liệu đầu vào không hợp lệ: " + err.Error()})
		return
	}

	if req.RelatedProducts == nil {
		req.RelatedProducts = []uint64{}
	}

	// 3. Gọi service cập nhật
	if _, err := services.CreateProduct(c, &req); err != nil {
		// Log lỗi chi tiết ra journalctl
		log.Printf("Lỗi tạo sản phẩm : %v", err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Tạo thất bại",
			"debug": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tạo sản phẩm thành công",
	})
}

func UpdateProduct(c *gin.Context) {
	var req models.ProductSaveRequest

	// ShouldBind giờ đây chỉ bind các trường đơn giản, không bị nổ lỗi trên mảng/struct
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu đầu vào không hợp lệ: " + err.Error()})
		return
	}

	id := c.Param("id")

	// Gọi service cập nhật
	if err := services.UpdateProduct(c, id, &req); err != nil {
		log.Printf("Lỗi cập nhật sản phẩm ID %s: %v", id, err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Cập nhật thất bại",
			"debug": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Cập nhật sản phẩm thành công",
	})
}

func DeleteProduct(c *gin.Context) {
	// ShouldBind tự động nhận diện Content-Type (JSON hoặc Multipart Form)
	var req models.Product

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(400, gin.H{"error": "Dữ liệu đầu vào không hợp lệ: " + err.Error()})
		return
	}

	id := c.Param("id")

	if req.RelatedProducts == nil {
		req.RelatedProducts = []uint64{}
	}

	// 3. Gọi service cập nhật
	if err := services.DeleteProduct(c, id); err != nil {
		// Log lỗi chi tiết ra journalctl
		log.Printf("Xóa sản phẩm ID %s: %v", id, err)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Xóa thất bại",
			"debug": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Xóa sản phẩm thành công",
	})
}

func ProductDetail(c *gin.Context) {
	idStr := c.Param("id")
	utils.LogSQL(idStr)

	productID, _ := strconv.ParseInt(idStr, 10, 64)

	product, err := services.GetProductDetail(c, productID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   product,
	})
}

func ProductBySlug(c *gin.Context) {
	slug := c.Param("slug")
	utils.LogSQL(slug)

	product, err := services.GetProductBySlug(c, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   product,
	})
}
