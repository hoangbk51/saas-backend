package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// Structural Model giả định cho Cart (Cần khớp với struct DB của bạn)
type Cart struct {
	ID           uint64 `gorm:"primaryKey"`
	Status       string `gorm:"default:'active'"` // 'active', 'completed', 'abandoned'
	SequenceStep int    `gorm:"default:0"`
	NextRetryAt  *time.Time
}

func HandleAbandonedCartTask(ctx context.Context, t *asynq.Task) error {
	var p AbandonedCartPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("lỗi unmarshal abandoned cart payload: %w", err)
	}

	utils.LogToFile("[ABANDONED CART WORKER] Process Step %d - CartID=%d Tenant=%s", p.Step, p.CartID, p.TenantID)

	// 1. Lấy DB của Tenant
	db, err := utils.GetTenantDBByTenantID(p.TenantID)
	if err != nil || db == nil {
		return fmt.Errorf("không thể lấy Tenant DB: %s, err: %v", p.TenantID, err)
	}
	// 2. Kiểm tra trạng thái thực tế của Giỏ hàng trong DB (Dùng database/sql chuẩn)
	var cartStatus string
	var sequenceStep int

	query := "SELECT status, sequence_step FROM carts WHERE id = ?"
	err = db.QueryRowContext(ctx, query, p.CartID).Scan(&cartStatus, &sequenceStep)
	if err != nil {
		utils.LogToFile("[ABANDONED CART] Giỏ hàng ID %d không tồn tại hoặc lỗi DB: %v -> Bỏ qua", p.CartID, err)
		return nil // Discard job
	}

	// 🛑 STOP CONDITION 1: Khách đã thanh toán thành công
	if cartStatus == "completed" {
		utils.LogToFile("[ABANDONED CART] CartID %d đã completed -> DỪNG GỬI MAIL", p.CartID)
		return nil
	}

	// 🛑 STOP CONDITION 2: Tránh gửi lặp step
	if sequenceStep >= p.Step {
		utils.LogToFile("[ABANDONED CART] CartID %d đã xử lý step %d trước đó -> Bỏ qua", p.CartID, p.Step)
		return nil
	}
	// 3. Chuẩn bị Data Render Email dựa trên Step
	templateName := "abandoned_cart_step1"
	emailData := map[string]any{
		"CartID": p.CartID,
	}

	// Logic phân nhánh Step & Coupon
	if p.Step == 2 {
		templateName = "abandoned_cart_step2"
		// 💡 Có thể kiểm tra giỏ hàng có hàng Sale không trước khi tạo coupon động
		emailData["CouponCode"] = "SAVE10" // Hoặc tự tạo Dynamic Coupon
	} else if p.Step == 3 {
		templateName = "abandoned_cart_step3"
		emailData["WarningText"] = "Giỏ hàng của bạn sẽ bị xóa sau 12h nữa!"
	}

	// 4. Tạo Gin Context giả lập để tái sử dụng MailService
	c, _ := gin.CreateTestContext(nil)
	req, _ := http.NewRequestWithContext(ctx, "GET", "/", nil)
	c.Request = req
	c.Set("db", db)
	c.Set("tenantId", p.TenantID)

	mailService := utils.NewMailConfig()
	if err := mailService.SendDynamicEmail(c, p.ToEmail, templateName, emailData); err != nil {
		utils.LogToFile("[ABANDONED CART FAIL] Lỗi gửi mail Step %d: %v", p.Step, err)
		return err // Trả về lỗi để Asynq Retry nếu cấu hình MaxRetry
	}

	// 5. Cập nhật trạng thái DB sau khi gửi mail thành công
	now := time.Now()
	var nextProcessAt *time.Time

	if p.Step == 1 || p.Step == 2 {
		t := now.Add(24 * time.Hour)
		nextProcessAt = &t
	}

	updateQuery := "UPDATE carts SET sequence_step = ?, next_retry_at = ? WHERE id = ?"
	_, err = db.ExecContext(ctx, updateQuery, p.Step, nextProcessAt, p.CartID)
	if err != nil {
		utils.LogToFile("[ABANDONED CART] Lỗi update DB CartID %d: %v", p.CartID, err)
	}

	// 6. 🚀 CHAINING JOB: Tự động Đẩy Job kế tiếp vào Asynq Queue (Nếu chưa hết Sequence)
	if p.Step < 3 {
		nextStep := p.Step + 1
		_ = EnqueueAbandonedCartRecovery(utils.AsynqClient, p.TenantID, p.CartID, p.ToEmail, nextStep, *nextProcessAt)
	}

	return nil
}
