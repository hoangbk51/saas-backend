package controllers

import (
	"go-saas/services"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// VirtualTryOn handles POST requests for AI garment try-on
// Requires customer authentication: khách hàng đã đăng nhập mới được sử dụng
func VirtualTryOn(c *gin.Context) {
	// 1. Kiểm tra xác thực Customer (yêu cầu khách hàng đã đăng nhập)
	customerIDVal, exists := c.Get("customerID")
	utils.LogToFile("customerIDVal")

	utils.LogToFile(customerIDVal)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Vui lòng đăng nhập để sử dụng tính năng thử đồ ảo (Customer authentication required)",
		})
		return
	}

	customerID, ok := customerIDVal.(int)
	if !ok || customerID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Tài khoản khách hàng không hợp lệ hoặc phiên đăng nhập đã hết hạn",
		})
		return
	}

	// 2. Parse request body
	var req services.VirtualTryOnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Dữ liệu gửi lên không đúng định dạng JSON. Cần có product_image và customer_image.",
			"details": err.Error(),
		})
		return
	}

	if req.ProductImage == "" || req.CustomerImage == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Vui lòng cung cấp đầy đủ ảnh sản phẩm (product_image) và ảnh khách hàng (customer_image)",
		})
		return
	}

	data, statusCode, err := services.ProcessVirtualTryOnService(c)
	if err != nil {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}
