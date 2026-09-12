package controllers

import (
	"fmt"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// POST /api/compare/add
func AddCompare(c *gin.Context) {
	// 1. Kiểm tra an toàn xem middleware auth đã set user_id chưa
	var customerID int
	if id, exists := c.Get("customerID"); exists {
		customerID = id.(int)
	} else {
		customerID = 0 // Guest
	}

	// Lấy IP Address của client
	ipAddress := c.ClientIP()
	utils.LogSQL(fmt.Sprintf("DEBUG: customerID=%d, IP=%s", customerID, ipAddress))

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
	utils.LogSQL(fmt.Sprintf("DEBUG: productID nhận được là %d", productID))

	// 3. Gọi service và truyền thêm ipAddress
	if err := services.AddCompare(c, customerID, ipAddress, productID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"status": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "message": "Đã thêm vào danh sách so sánh"})
}

// GET /api/compare
func ListCompares(c *gin.Context) {
	var customerID int
	if id, exists := c.Get("customerID"); exists {
		customerID = id.(int)
	} else {
		customerID = 0 // Guest
	}

	ipAddress := c.ClientIP()

	list, err := services.GetCompares(c, customerID, ipAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": list})
}

// DELETE /api/compare/remove/:product_id
func DeleteCompare(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": "product_id không hợp lệ"})
		return
	}

	var customerID int
	if id, exists := c.Get("customerID"); exists {
		customerID = id.(int)
	} else {
		customerID = 0 // Guest
	}

	ipAddress := c.ClientIP()

	if err := services.RemoveCompare(c, customerID, ipAddress, productID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "message": "Đã xóa khỏi danh sách so sánh"})
}
