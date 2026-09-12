package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetUnmappedProductsHandler GET /api/v1/channel/unmapped
func GetUnmappedProductsHandler(c *gin.Context) {

	//userID := c.GetUint64("user_id")

	//channelType := c.Query("channel_type") // 'SHOPEE' hoặc 'TIKTOK'
	storeID, _ := strconv.ParseUint(c.Query("store_id"), 10, 64)

	list, err := services.GetUnmappedProducts(c, "TIKTOK", storeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": list,
	})
}

// ManualMapSKUHandler POST /api/v1/channel/map
func ManualMapSKUHandler(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi CSDL"})
		return
	}
	userID := c.GetUint64("user_id")

	var req models.ManualMapSKURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Tham số truyền vào không hợp lệ"})
		return
	}

	if err := services.ManualMapSKU(db, userID, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Ghép nối sản phẩm thành công",
	})
}
