package controllers

import (
	"fmt"
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// POST /api/v1/channel/sync-orders
func SyncOrders(c *gin.Context) {
	var req services.SyncOrdersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}

	count, err := services.SyncOrdersFromChannel(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": fmt.Sprintf("Đã đồng bộ thành công %d đơn hàng từ Sàn!", count),
		"data": gin.H{
			"synced_count": count,
		},
	})
}
