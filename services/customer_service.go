package services

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"go-saas/tasks"
	"go-saas/utils"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

func LogoutCustomer(c *gin.Context, authHeader string) error {
	db, err := utils.GetDBFromContext(c)

	// 1. Kiểm tra định dạng Header
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return fmt.Errorf("invalid authorization header")
	}
	rawToken := strings.TrimPrefix(authHeader, "Bearer ")

	// 2. Xử lý logic Sanctum (id|plainTextToken)
	parts := strings.SplitN(rawToken, "|", 2)
	tokenToHash := parts[0]
	if len(parts) == 2 {
		tokenToHash = parts[1]
	}

	// 3. Hash SHA-256 theo đúng chuẩn Laravel Sanctum
	h := sha256.New()
	h.Write([]byte(tokenToHash))
	hashedToken := hex.EncodeToString(h.Sum(nil))

	// 4. Dùng sqlx để thực thi DELETE vật lý
	query := "DELETE FROM personal_access_tokens WHERE token = ?"
	result, err := db.Exec(query, hashedToken)
	if err != nil {
		return err
	}

	// Kiểm tra xem có bản ghi nào bị xóa không (nếu = 0 tức là token sai hoặc đã xóa rồi)
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token not found or already invalidated")
	}

	return nil
}

func VerifyCustomer(c *gin.Context, token string) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil || tenantDB == nil {
		return fmt.Errorf("không thể kết nối database tenant")
	}

	// 1. Kiểm tra xem token có tồn tại & lấy thông tin Customer
	var customer struct {
		ID    uint64
		Email string
		Name  string
	}

	checkQuery := `
		SELECT id, email, name
		FROM customers 
		WHERE verification_token = ? AND deleted_at IS NULL 
		LIMIT 1
	`
	err = tenantDB.QueryRow(checkQuery, token).Scan(
		&customer.ID,
		&customer.Email,
		&customer.Name,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("Mã xác thực không hợp lệ hoặc đã hết hạn")
		}
		return err
	}

	// 2. Cập nhật trạng thái active = 1 và xóa verification_token (để tránh verify lại)
	updateQuery := `
		UPDATE customers 
		SET active = 1, 
		    verification_token = NULL, 
		    updated_at = NOW() 
		WHERE id = ?
	`
	_, err = tenantDB.Exec(updateQuery, customer.ID)
	if err != nil {
		return err
	}

	// =========================================================================
	// 3. ĐẨY ASYNQ TASK: GỬI WELCOME EMAIL SAU KHIM XÁC THỰC THÀNH CÔNG
	// =========================================================================
	if utils.AsynqClient != nil && customer.Email != "" {
		// Lấy tên cửa hàng và domain từ setting (nếu có)
		storeName := utils.GetSetting(c, "store_name", "Cửa hàng của chúng tôi")
		storeURL := utils.GetSetting(c, "store_url", "#")

		customerName := customer.Name
		if customerName == " " || customerName == "" {
			customerName = "Khách hàng"
		}

		// Data thay thế vào Template
		reps := map[string]any{
			"CustomerName": customerName,
			"StoreName":    storeName,
			"StoreURL":     storeURL,
			"CouponCode":   "WELCOME10", // Có thể lấy động từ setting hoặc để cố định
		}

		// Enqueue Task gửi email bất đồng bộ
		errEnq := tasks.EnqueueDynamicEmail(utils.AsynqClient, c.GetString("tenantId"), customer.Email, "customer_welcome", reps)
		if errEnq != nil {
			log.Printf("[Warning] Không thể enqueue Welcome Email cho customer ID %d: %v", customer.ID, errEnq)
		}
	}

	return nil
}
