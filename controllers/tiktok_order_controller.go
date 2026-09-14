package controllers

import (
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SyncTikTokOrdersHandler godoc
// POST /api/v1/tiktok/sync-orders

func SearchTikTokOrdersHandler(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/tiktok/order/search?page_size=%s",
		getTikTokMockBase(c), c.DefaultQuery("page_size", "10"))

	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)

	proxyTikTokMockResponse(c, "POST", mockURL, body)
}

// SyncTikTokOrdersHandler tiếp nhận yêu cầu đồng bộ đơn hàng TikTok về Database
func SyncTikTokOrdersHandler(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi kết nối CSDL"})
		return
	}
	userID := c.GetUint64("user_id")

	var reqBody map[string]interface{}
	_ = c.ShouldBindJSON(&reqBody)
	pageSize := c.DefaultQuery("page_size", "10")

	// 1. Gọi Service lấy dữ liệu đơn hàng từ TikTok
	mockBase := getTikTokMockBase(c)
	orders, err := services.FetchTikTokOrdersFromAPI(c, mockBase, pageSize, reqBody)
	utils.LogToFile(orders)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}

	if len(orders) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Không có đơn hàng mới nào", "synced_count": 0})
		return
	}

	// 2. Mở Transaction CSDL để lưu dữ liệu
	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi khởi tạo Transaction"})
		return
	}
	defer tx.Rollback()

	syncedCount := 0
	channelID := 2 // ID Kênh TikTok Shop trong hệ thống SaaS

	for _, orderDetail := range orders {
		if err := services.SaveOrUpdateTikTokOrder(tx, userID, channelID, orderDetail); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": fmt.Sprintf("Lỗi lưu đơn %s: %s", orderDetail.ID, err.Error())})
			return
		}
		syncedCount++
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi commit CSDL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":         0,
		"message":      "Đồng bộ đơn hàng TikTok thành công",
		"synced_count": syncedCount,
		"data":         orders,
	})
}

// TikTokWebhookHandler tiếp nhận thông báo tự động từ TikTok
// POST /tiktok/webhook
func TikTokWebhookHandler(c *gin.Context) {
	var payload models.TikTokWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Payload không hợp lệ"})
		return
	}

	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Database error"})
		return
	}

	// Xử lý Event dựa trên type
	switch payload.Type {
	case 1: // Thay đổi trạng thái đơn hàng
		updateQuery := "UPDATE `orders` SET `order_status_id` = ?, `updated_at` = NOW() WHERE `order_number` = ?"
		_, err := db.Exec(updateQuery, services.MapTikTokStatusToOrderID(payload.Data.OrderStatus), payload.Data.OrderID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi cập nhật trạng thái đơn"})
			return
		}
	}

	// TikTok yêu cầu phản hồi HTTP 200 OK với JSON code 0
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}
