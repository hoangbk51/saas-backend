package tasks

import (
	"encoding/json"
	"fmt"
	"go-saas/utils"
	"time"

	"github.com/hibiken/asynq"
)

const TypeOrderPaidEmail = "email:order_paid"

// Payload chứa thông tin tối thiểu cần thiết để Worker xử lý
type OrderPaidEmailPayload struct {
	OrderID  int64  `json:"order_id"`
	TenantID string `json:"tenant_id"`
}

// EnqueueOrderPaidEmail enqueue một job gửi mail vào Redis Queue
func EnqueueOrderPaidEmail(client *asynq.Client, orderID int64, tenantID string) error {
	payload, err := json.Marshal(OrderPaidEmailPayload{
		OrderID:  orderID,
		TenantID: tenantID,
	})
	if err != nil {
		return fmt.Errorf("lỗi marshal payload: %v", err)
	}

	// Cấu hình Task: Thử lại tối đa 5 lần nếu lỗi, timeout 30s mỗi lượt
	task := asynq.NewTask(TypeOrderPaidEmail, payload,
		asynq.MaxRetry(5),
		asynq.Timeout(30*time.Second),
		asynq.Queue("emails"), // Đặt tên Queue riêng biệt nếu muốn
	)

	info, err := client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("không thể enqueue email task: %v", err)
	}

	utils.LogToFile("[Asynq] Enqueued task ID=%s cho Order #%d", info.ID, orderID)
	return nil
}
