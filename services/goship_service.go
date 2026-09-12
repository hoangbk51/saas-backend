package services

import (
	"fmt"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

// GoShipWebhookPayload đại diện cho struct dữ liệu event từ GoShip
type GoShipWebhookPayload struct {
	Code       string `json:"code"`        // Mã vận đơn GoShip (VD: "56GDG8F")
	Status     string `json:"status"`      // Trạng thái chuỗi (VD: "delivering", "delivered")
	StatusCode int    `json:"status_code"` // Mã trạng thái số
	Note       string `json:"note"`        // Ghi chú lịch sử hành trình
	UpdatedAt  string `json:"updated_at"`  // Thời gian cập nhật
}

func ProcessWebhookEvent(c *gin.Context, payload GoShipWebhookPayload) error {
	utils.LogToFile(payload)
	if payload.Code == "" {
		return fmt.Errorf("tracking code rỗng")
	}

	// 1. Kết nối DB Central / Tenant
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("lỗi kết nối DB: %w", err)
	}

	// 2. Mapping trạng thái
	appStatus := mapGoShipStatusToApp(payload.Status, payload.StatusCode)

	// 3. Update CSDL
	query := `
		UPDATE orders 
		SET order_status_id = ?
		WHERE tracking_id = ? 
	`
	res, err := db.ExecContext(c, query, appStatus, payload.Code)
	if err != nil {
		return fmt.Errorf("lỗi update DB cho đơn %s: %w", payload.Code, err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		utils.LogToFile(fmt.Sprintf("[GoShip Webhook] Không tìm thấy đơn hàng với mã: %s", payload.Code))
	} else {
		utils.LogToFile(fmt.Sprintf("[GoShip Webhook] Cập nhật thành công đơn %s -> Status: %s", payload.Code, appStatus))
	}

	return nil
}

// Helper mapping trạng thái
func mapGoShipStatusToApp(status string, statusCode int) int {
	switch status {
	case "picking", "ready_to_pick":
		return 10
	case "delivering", "storing":
		return 11
	case "delivered":
		return 12
	case "returning", "returned":
		return 13
	case "cancelled":
		return 14
	default:
		return 15
	}
}
