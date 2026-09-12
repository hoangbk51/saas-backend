package tasks

import (
	"encoding/json"
	"fmt"
	"time"

	"go-saas/utils"

	"github.com/hibiken/asynq"
)

const TypeSendDynamicEmail = "email:send_dynamic"

type DynamicEmailPayload struct {
	TenantID     string         `json:"tenant_id"`
	ToEmail      string         `json:"to_email"`
	TemplateName string         `json:"template_name"` // "order_complete", "verify_registration", "order_confirm", ...
	Data         map[string]any `json:"data"`          // Map dữ liệu render HTML
}

// EnqueueDynamicEmail dùng chung cho MỌI loại mail
func EnqueueDynamicEmail(client *asynq.Client, tenantID string, toEmail string, templateName string, data map[string]any) error {
	utils.LogToFile("payload tenantID: '%s'", tenantID)

	payload, err := json.Marshal(DynamicEmailPayload{
		TenantID:     tenantID,
		ToEmail:      toEmail,
		TemplateName: templateName,
		Data:         data,
	})
	if err != nil {
		return fmt.Errorf("lỗi marshal payload: %v", err)
	}

	utils.LogToFile("[DEBUG ENQUEUE] TASK TYPE: '%s'", TypeSendDynamicEmail)

	task := asynq.NewTask(TypeSendDynamicEmail, payload,
		asynq.MaxRetry(5),
		asynq.Timeout(30*time.Second),
		asynq.Queue("emails"),
	)

	info, err := client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("không thể enqueue email task: %v", err)
	}

	utils.LogToFile("[Asynq] Enqueued mail %s (Template: %s) ID=%s Tenant=%s", toEmail, templateName, info.ID, tenantID)
	return nil
}
