package controllers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

// GetUnreadNotificationsHandler GET /api/v1/notifications/unread?limit=10
func GetUnreadNotificationsHandler(c *gin.Context) {
	// Lấy thông tin admin đăng nhập từ Middleware Auth (Ví dụ ID = 1, Type = "Admin")
	adminID := c.GetUint64("adminID") // hoặc c.GetUint64("user_id")
	notifiableType := "App\\User"     // hoặc "App\\Models\\User" tùy convention của bạn

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	result, err := services.GetUnreadNotifications(c, notifiableType, adminID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Không thể lấy danh sách thông báo: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// MarkAsReadHandler POST/PUT /api/v1/notifications/read
func MarkAsReadHandler(c *gin.Context) {
	adminID := c.GetUint64("adminID")
	notifiableType := "App\\User"

	var req models.MarkReadRequest
	// Bind JSON nếu có body truyền lên, nếu không truyền body -> coi như mark all
	_ = c.ShouldBindJSON(&req)

	rowsAffected, err := services.MarkNotificationsAsRead(c, notifiableType, adminID, req.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Cập nhật trạng thái thất bại: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Đã cập nhật trạng thái thông báo thành công",
		"data": gin.H{
			"updated_count": rowsAffected,
		},
	})
}
func StreamAdminNotifications(c *gin.Context) {
	// 1. Cấu hình Header cho kết nối SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// 2. Lấy TenantID (string) và AdminID (uint64) từ Middleware Auth
	tenantID := c.GetString("tenantId")

	adminVal, exists := c.Get("adminID")
	if !exists || tenantID == "" {
		c.SSEvent("error", "Unauthorized")
		return
	}

	adminID, ok := adminVal.(uint64)
	if !ok || adminID == 0 {
		c.SSEvent("error", "Unauthorized")
		return
	}

	// 3. Subscribe đúng Channel cá nhân của User trên Redis (dùng %d cho uint64)
	channelName := fmt.Sprintf("tenant:%s:user:%d", tenantID, adminID)

	if utils.RedisClient == nil {
		c.SSEvent("error", "Redis client unavailable")
		return
	}

	pubsub := utils.RedisClient.Subscribe(c.Request.Context(), channelName)
	//utils.LogToFile("SSE Subscribed Channel: " + channelName)
	defer pubsub.Close()

	redisChan := pubsub.Channel()

	// 4. Lắng nghe sự kiện từ Redis và Push về Client qua SSE Stream
	c.Stream(func(w io.Writer) bool {
		select {
		// Kết nối đóng (User đóng trình duyệt / chuyển trang) -> Thoát Goroutine
		case <-c.Request.Context().Done():
			return false

		// Nhận tin nhắn mới từ Redis Channel cá nhân -> Bắn về Browser
		case msg, ok := <-redisChan:
			if !ok {
				return false
			}
			c.SSEvent("new_order", msg.Payload)

			// Flush buffer để truyền ngay dữ liệu xuống Client
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			return true
		}
	})
}
