package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	"go-saas/models"
	"go-saas/utils"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// HandleOrderPaidEmailTask nhận task từ Asynq và gọi Mail Service
func HandleOrderPaidEmailTask(ctx context.Context, t *asynq.Task) error {

	var p OrderPaidEmailPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("lỗi unmarshal payload: %v", err)
	}

	c, _ := gin.CreateTestContext(nil)
	db, err := utils.GetDBFromContext(c) // Hàm lấy DB instance theo TenantID của bạn

	c.Set("tenantId", p.TenantID)
	c.Set("db", db)
	// Tạo Request giả lập để GetSetting đọc c.Request.Context() không bị panic
	c.Request, _ = http.NewRequestWithContext(ctx, "GET", "/", nil)

	// 1. Lấy thông tin Order & Items & Totals từ DB
	var order models.Order
	err = db.QueryRow("SELECT id, order_number, email, grand_total, billing_first_name, billing_last_name FROM orders WHERE id = ?", p.OrderID).
		Scan(&order.ID, &order.OrderNumber, &order.Email, &order.GrandTotal, &order.BillingFirstName, &order.BillingLastName)
	if err != nil {
		return fmt.Errorf("không tìm thấy order #%d: %v", p.OrderID, err)
	}

	var items []models.OrderItem
	_ = db.Select(&items, "SELECT item_description, quantity, unit_price FROM order_items WHERE order_id = ?", p.OrderID)

	var totals []models.OrderTotal
	_ = db.Select(&totals, "SELECT title, value FROM order_totals WHERE order_id = ? ORDER BY sort_order ASC", p.OrderID)

	reps := map[string]any{
		"CustomerName": order.BillingFirstName + " " + order.BillingLastName,
		"OrderNumber":  order.OrderNumber,
		"GrandTotal":   order.GrandTotal,
		"Items":        items,
		"Totals":       totals,
	}

	mailService := utils.NewMailConfig()

	// 2. Gửi mail cho Khách hàng
	if order.Email != "" {
		_ = mailService.SendDynamicEmail(c, order.Email, "order_complete", reps)
	}

	// 3. Lấy email Admin dùng utils.GetSetting (Fallback sang .env nếu rỗng)
	adminEmail := utils.GetSetting(c, "store_email", os.Getenv("ADMIN_EMAIL"))

	if adminEmail != "" {
		_ = mailService.SendDynamicEmail(c, adminEmail, "admin_order_paid", reps)
		utils.LogToFile("[Asynq Worker] Đã gửi mail admin thành công đến %s", adminEmail)
	}

	return nil
}
