package services

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// AddOrderHistoryService thêm lịch sử đơn hàng và cập nhật trạng thái đơn hàng
func AddOrderHistoryService(c *gin.Context, paramOrderID string, req models.CreateOrderHistoryRequest) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	// 1. Lấy orderID ưu tiên từ URL param hoặc từ payload

	var orderID int64
	if paramOrderID != "" {
		orderID, _ = strconv.ParseInt(paramOrderID, 10, 64)
	}
	if orderID == 0 {
		orderID = req.OrderID
	}

	if orderID == 0 {
		return gin.H{"error": "ID đơn hàng không hợp lệ"}, http.StatusBadRequest, fmt.Errorf("invalid order_id")
	}

	if err != nil || orderID == 0 {
		return gin.H{"error": "ID đơn hàng không hợp lệ"}, http.StatusBadRequest, fmt.Errorf("invalid order_id")
	}

	// 2. Mở Transaction
	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo transaction"}, http.StatusInternalServerError, err
	}
	defer tx.Rollback() // Tự động rollback nếu xảy ra lỗi trước Commit

	now := time.Now()

	// 3. Insert lịch sử mới vào bảng order_histories
	insertQuery := `
		INSERT INTO order_histories (order_id, order_status_id, comment, notify, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	res, err := tx.ExecContext(c.Request.Context(), insertQuery,
		orderID,
		req.Status, // order_status_id từ payload (Ví dụ: 2 - Waiting for payment)
		req.Note,   // comment
		0,          // notify mặc định 0
		now,
		now,
	)
	if err != nil {
		return gin.H{"error": "Lỗi thêm lịch sử đơn hàng: " + err.Error()}, http.StatusInternalServerError, err
	}

	historyID, _ := res.LastInsertId()

	// 4. Đồng bộ cập nhật trạng thái mới nhất vào bảng orders
	updateOrderQuery := `
		UPDATE orders 
		SET order_status_id = ?, updated_at = ? 
		WHERE id = ? AND deleted_at IS NULL`

	_, err = tx.ExecContext(c.Request.Context(), updateOrderQuery, req.Status, now, orderID)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật trạng thái đơn hàng: " + err.Error()}, http.StatusInternalServerError, err
	}

	// 5. Commit Transaction
	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi commit transaction"}, http.StatusInternalServerError, err
	}

	orderData := map[string]interface{}{
		"order_id":   orderID,
		"to_status":  req.Status,
		"history_id": historyID,
	}
	var notifiableID uint64 = 0
	notify := models.Notification{
		Type:           "ORDER_STATUS_CHANGE",
		NotifiableType: "App\\Models\\Tenant\\User",
		NotifiableID:   &notifiableID, // Để nil vì thông báo này gửi cho cả nhóm Admin

		// Gán role hoặc danh sách permission nhận thông báo này:
		//TargetRole:        utils.StringPtr("ORDER_MANAGER"),
		TargetPermissions: []string{"notify_order_status_change"}, // (Chọn 1 trong 2 hoặc truyền cả 2)

		Icon:       utils.StringPtr("shopping-cart"),
		ActionText: utils.StringPtr("Xem đơn hàng"),
		ActionURL:  utils.StringPtr(fmt.Sprintf("/admin/orders/%d", orderID)),
		Message:    utils.StringPtr(fmt.Sprintf("Đơn hàng cập nhật trạng thái #%d", orderID)),
		Data:       orderData,
	}

	// Gọi Goroutine bất đồng bộ
	tenantID := c.GetString("tenantId")

	go func(tID string, tDB *sqlx.DB) {
		err := CreateAndBroadcastToTargetUsers(context.Background(), tDB, tID, notify)
		if err != nil {
			utils.LogToFile("[Notification Error] %v", err)
		}
	}(tenantID, tenantDB)

	return gin.H{
		"message": "Thêm lịch sử đơn hàng thành công",
		"data": gin.H{
			"id":              historyID,
			"order_id":        orderID,
			"order_status_id": req.Status,
			"comment":         req.Note,
			"created_at":      now.Format("2006-01-02 15:04:05"),
		},
	}, http.StatusOK, nil
}
