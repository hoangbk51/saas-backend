package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

func HandleDynamicEmailTask(ctx context.Context, t *asynq.Task) error {
	utils.LogToFile("HandleDynamicEmailTask")

	var p DynamicEmailPayload

	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		utils.LogToFile("lỗi unmarshal payload")

		return fmt.Errorf("lỗi unmarshal payload: %w", err)
	}

	utils.LogToFile(
		"[EMAIL WORKER] Start tenant=%s email=%s template=%s",
		p.TenantID,
		p.ToEmail,
		p.TemplateName,
	)

	// 1. Bắt buộc phải có tenantID
	if p.TenantID == "" {
		return fmt.Errorf("email task không có tenantID")
	}

	// 2. Lấy đúng DB của tenant
	db, err := utils.GetTenantDBByTenantID(p.TenantID)
	if err != nil {
		return fmt.Errorf(
			"không lấy được tenant DB tenant=%s: %w",
			p.TenantID,
			err,
		)
	}

	if db == nil {
		return fmt.Errorf(
			"tenant DB nil tenant=%s",
			p.TenantID,
		)
	}

	utils.LogToFile(
		"[EMAIL WORKER] Tenant DB OK tenant=%s",
		p.TenantID,
	)

	// 3. Tạo Gin Context giả để giữ nguyên kiến trúc cũ
	c, _ := gin.CreateTestContext(nil)

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"/",
		nil,
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	c.Request = req

	// 4. Đây là điểm quan trọng:
	// Set DB TRƯỚC khi gọi GetDBFromContext()
	c.Set("db", db)

	// Nếu code khác của hệ thống cần tenantId
	c.Set("tenantId", p.TenantID)

	// 5. Giữ nguyên MailService hiện tại
	mailService := utils.NewMailConfig()

	if err := mailService.SendDynamicEmail(
		c,
		p.ToEmail,
		p.TemplateName,
		p.Data,
	); err != nil {
		utils.LogToFile(
			"[Asynq Worker Error] tenant=%s email=%s template=%s error=%v",
			p.TenantID,
			p.ToEmail,
			p.TemplateName,
			err,
		)

		return err
	}

	utils.LogToFile(
		"[Asynq Worker Success] tenant=%s email=%s template=%s",
		p.TenantID,
		p.ToEmail,
		p.TemplateName,
	)

	return nil
}
