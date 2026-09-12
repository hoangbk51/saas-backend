package services

import (
	"context"
	"fmt"
	"go-saas/models"
	"go-saas/tasks"
	"go-saas/utils"
	"os"

	"github.com/gin-gonic/gin"
)

type EventNotificationSetting struct {
	InApp   bool `json:"in_app"`
	Email   bool `json:"email"`
	SMS     bool `json:"sms"`
	ZaloZNS bool `json:"zalo_zns"`
}

type NotificationSettings map[string]EventNotificationSetting

func UpdateShippingMethod(c *gin.Context, shippingCode string, clientIP string) (*models.RecalculateResult, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	// 1. Tìm CartID hiện tại của Guest (hoặc User)
	var cartID int64
	queryGet := `SELECT id FROM carts WHERE ip_address = ? AND customer_id IS NULL AND deleted_at IS NULL LIMIT 1`
	err = db.QueryRow(queryGet, clientIP).Scan(&cartID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy giỏ hàng: %v", err)
	}

	// 2. Cập nhật phương thức vận chuyển mới
	// Bạn có thể mở rộng query này để cập nhật thêm country_id, state_id nếu cần
	queryUpdate := `UPDATE carts SET shipping_method_code = ?, updated_at = NOW() WHERE id = ?`
	_, err = db.Exec(queryUpdate, shippingCode, cartID)
	if err != nil {
		return nil, fmt.Errorf("lỗi cập nhật phương thức vận chuyển: %v", err)
	}

	// 3. Lấy lại object Cart đầy đủ
	cart, err := GetCartById(c, cartID)
	if err != nil {
		return nil, err
	}

	// 4. Gọi hàm Recalculate (Hàm này sẽ gọi calculateShippingRate bên trong)
	// để tính lại phí ship dựa trên shipping_method_code mới
	return CartRecalculate(c, cart)
}

func CheckoutCallBack(c *gin.Context, code string) error {
	db, _ := utils.GetDBFromContext(c)

	// Cập nhật Transaction status = 1 (Thành công)
	query := `UPDATE transactions SET status = 1, updated_at = NOW() WHERE code = ?`
	utils.LogSQL(query, code)

	_, err := db.Exec(query, code)

	transaction, _ := GetTransactionByCode(c, code)
	queryOrder := `UPDATE orders SET payment_status = 3, updated_at = NOW() WHERE id = ?`
	utils.LogSQL(queryOrder, transaction.OrderID)

	_, err = db.Exec(queryOrder, transaction.OrderID)

	//go SendOrderPaidSucccessEmail(c.Copy(), transaction.OrderID)
	_ = SendOrderPaidSucccessEmail(c, transaction.OrderID)
	//todo xóa cart sau khi order thanh toán
	return err
}

func SendOrderPaidSucccessEmail(c *gin.Context, orderID int64) error {
	tenantID := c.GetString("tenantId")
	db, _ := utils.GetDBFromContext(c)

	// ⚡ Chỉ 1 dòng duy nhất để lấy setting cho event payment_success
	paymentSetting := utils.GetNotificationSettingByEvent(c, "payment_success")

	if paymentSetting.Email && utils.AsynqClient != nil {
		// 1.1 Query thông tin Order & Items để chuẩn bị payload reps
		var order models.Order
		_ = db.QueryRow("SELECT id, order_number, email, grand_total, billing_first_name, billing_last_name FROM orders WHERE id = ?", orderID).
			Scan(&order.ID, &order.OrderNumber, &order.Email, &order.GrandTotal, &order.BillingFirstName, &order.BillingLastName)

		var items []models.OrderItem
		_ = db.Select(&items, "SELECT item_description, quantity, unit_price FROM order_items WHERE order_id = ?", orderID)

		var totals []models.OrderTotal
		_ = db.Select(&totals, "SELECT title, value FROM order_totals WHERE order_id = ? ORDER BY sort_order ASC", orderID)

		reps := map[string]any{
			"CustomerName": order.BillingFirstName + " " + order.BillingLastName,
			"OrderNumber":  order.OrderNumber,
			"GrandTotal":   order.GrandTotal,
			"Items":        items,
			"Totals":       totals,
		}

		//  TASK 1: Gửi Email cho Khách hàng (Order Complete)
		if order.Email != "" {
			_ = tasks.EnqueueDynamicEmail(utils.AsynqClient, c.GetString("tenantId"), order.Email, "customer_order_paid", reps)
		}

		//TASK 2: Gửi Email cho Admin (Notification Admin)
		adminEmail := utils.GetSetting(c, "store_email", os.Getenv("ADMIN_EMAIL"))
		if adminEmail != "" {
			_ = tasks.EnqueueDynamicEmail(utils.AsynqClient, c.GetString("tenantId"), adminEmail, "admin_order_paid", reps)
		}
	}

	// 🔔 Gửi Notify Admin In-App
	if paymentSetting.InApp {
		go func(tID string, oID int64) {
			var notifiableID uint64 = 0
			notify := models.Notification{
				Type:              "ORDER_PAID_SUCCESS",
				NotifiableType:    "App\\Models\\Tenant\\User",
				NotifiableID:      &notifiableID,
				TargetPermissions: []string{"notify_order"},
				Icon:              utils.StringPtr("check-circle"),
				ActionText:        utils.StringPtr("Xem đơn hàng"),
				ActionURL:         utils.StringPtr(fmt.Sprintf("/admin/orders/%d", oID)),
				Message:           utils.StringPtr(fmt.Sprintf("Đơn hàng #%d đã được thanh toán thành công!", oID)),
			}

			errNotify := CreateAndBroadcastToTargetUsers(context.Background(), db, tID, notify)
			if errNotify != nil {
				utils.LogToFile("[Notification Admin Error] %v", errNotify)
			}
		}(tenantID, orderID)
	}

	return nil
}
