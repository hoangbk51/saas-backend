package tasks

import (
	"encoding/json"
	"fmt"
	"time"

	"go-saas/utils"

	"github.com/hibiken/asynq"
)

const TypeAbandonedCartRecovery = "cart:abandoned_recovery"

type AbandonedCartPayload struct {
	TenantID string `json:"tenant_id"`
	CartID   uint64 `json:"cart_id"`
	ToEmail  string `json:"to_email"`
	Step     int    `json:"step"` // 1: Mail 1 (Sau 1h), 2: Mail 2 (Sau 24h), 3: Mail 3 (Sau 48h)
}

// EnqueueAbandonedCartRecovery đẩy job hẹn giờ vào Asynq
func EnqueueAbandonedCartRecovery(client *asynq.Client, tenantID string, cartID uint64, toEmail string, step int, processAt time.Time) error {
	payload, err := json.Marshal(AbandonedCartPayload{
		TenantID: tenantID,
		CartID:   cartID,
		ToEmail:  toEmail,
		Step:     step,
	})
	if err != nil {
		return fmt.Errorf("lỗi marshal abandoned cart payload: %w", err)
	}

	// Tính thời gian delay từ bây giờ tới thời điểm processAt
	delay := time.Until(processAt)
	if delay < 0 {
		delay = 0 // Nếu thời gian đã qua thì chạy ngay
	}

	task := asynq.NewTask(TypeAbandonedCartRecovery, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
		asynq.Queue("emails"),
		asynq.ProcessIn(delay), // 💡 Kỹ thuật Delayed Queue của Asynq
	)

	info, err := client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("không thể enqueue abandoned cart task: %w", err)
	}

	utils.LogToFile("[Asynq] Enqueued Abandoned Cart Step %d for Cart %d (Tenant: %s) TaskID=%s. Chạy lúc: %s",
		step, cartID, tenantID, info.ID, processAt.Format("2006-01-02 15:04:05"))

	return nil
}
