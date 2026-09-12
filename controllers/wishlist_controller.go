package controllers

import (
	"fmt"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// POST /api/wishlist/add
func AddWishlist(c *gin.Context) {
	// 1. Lấy thông tin định danh (Customer ID nếu có, và IP Address)
	var customerID int
	if id, exists := c.Get("customerID"); exists {
		customerID = id.(int)
	} else {
		customerID = 0 // Guest
	}

	// Lấy IP Address của client
	ipAddress := c.ClientIP()

	utils.LogSQL(fmt.Sprintf("DEBUG: customerID=%d, IP=%s", customerID))

	var data map[string]interface{}

	// 2. Bind dữ liệu từ Body vào map
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(400, gin.H{"error": "JSON không hợp lệ"})
		return
	}

	if data["product_id"] == nil {
		c.JSON(400, gin.H{"error": "Thiếu product_id"})
		return
	}
	productID := uint64(data["product_id"].(float64))

	// 3. Gọi Service truyền thêm ipAddress
	if err := services.AddWishlist(c, customerID, ipAddress, productID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "message": "Đã thêm vào yêu thích"})
}

// GET /api/wishlist
func ListWishlists(c *gin.Context) {
	// 1. Kiểm tra xem có customerID từ middleware không (an toàn, không crash)
	var customerID int
	if id, exists := c.Get("customerID"); exists {
		customerID = id.(int)
	} else {
		customerID = 0 // Guest
	}

	// 2. Lấy IP Address của Client
	ipAddress := c.ClientIP()

	// 3. Gọi service với cả customerID và ipAddress
	list, err := services.GetWishlists(c, customerID, ipAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": list})
}

// DELETE /api/wishlist/remove/:product_id
func DeleteWishlist(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "product_id không hợp lệ"})
		return
	}

	// 1. Kiểm tra an toàn xem user đã đăng nhập chưa
	var customerID int
	if id, exists := c.Get("customerID"); exists {
		customerID = id.(int)
	} else {
		customerID = 0 // Guest
	}

	// 2. Lấy IP Address của khách mạng
	ipAddress := c.ClientIP()

	// 3. Gọi service và truyền thêm ipAddress
	if err := services.RemoveWishlist(c, customerID, ipAddress, productID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "message": "Đã xóa khỏi yêu thích"})
}
