package controllers

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"go-saas/services"
	"go-saas/utils"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Request Structs
type RequestSendOTP struct {
	Phone string `json:"phone" binding:"required"`
}

type RequestVerifyOTP struct {
	Phone   string `json:"phone" binding:"required"`
	OTPCode string `json:"otp_code" binding:"required"`
}

// GenerateRandomOTP tạo chuỗi số ngẫu nhiên N chữ số
func GenerateRandomOTP(length int) (string, error) {
	const digits = "0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		result[i] = digits[num.Int64()]
	}
	return string(result), nil
}

// 1. API Yêu cầu gửi OTP
// POST /api/v1/phone/send-otp
func SendPhoneOTP(c *gin.Context) {
	var req RequestSendOTP
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Số điện thoại không hợp lệ"})
		return
	}

	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi kết nối cơ sở dữ liệu"})
		return
	}

	// Tạo mã OTP 6 chữ số
	otpCode, err := GenerateRandomOTP(6)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tạo mã OTP"})
		return
	}

	expiresAt := time.Now().Add(5 * time.Minute) // OTP có hiệu lực trong 5 phút

	// Lưu hoặc cập nhật OTP vào DB (Nếu SĐT đã gửi OTP trước đó, vô hiệu hóa OTP cũ)
	query := `
		INSERT INTO phone_verifications (phone, otp_code, is_verified, expires_at)
		VALUES (?, ?, 0, ?)
	`
	_, err = db.ExecContext(c.Request.Context(), query, req.Phone, otpCode, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lưu thông tin OTP"})
		return
	}

	// Gửi tin nhắn qua Twilio
	smsContent := fmt.Sprintf("Ma xac thuc OTP cua ban la: %s. Ma co hieu luc trong 5 phut.", otpCode)
	if err := services.SendTwilioSMS(c, req.Phone, smsContent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Gửi SMS thất bại: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Mã OTP đã được gửi thành công",
	})
}

// 2. API Verify OTP
// POST /api/v1/phone/verify-otp
func VerifyPhoneOTP(c *gin.Context) {
	var req RequestVerifyOTP
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu yêu cầu không hợp lệ"})
		return
	}

	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi kết nối cơ sở dữ liệu"})
		return
	}

	// Kiểm tra mã OTP trong cơ sở dữ liệu
	var id int64
	var expiresAt time.Time

	checkQuery := `
		SELECT id, expires_at 
		FROM phone_verifications 
		WHERE phone = ? AND otp_code = ? AND is_verified = 0
		ORDER BY id DESC 
		LIMIT 1
	`
	err = db.QueryRowContext(c.Request.Context(), checkQuery, req.Phone, req.OTPCode).Scan(&id, &expiresAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã OTP không chính xác hoặc đã được sử dụng"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi xác thực dữ liệu"})
		return
	}

	// Kiểm tra thời gian hết hạn
	if time.Now().After(expiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã OTP đã hết hạn"})
		return
	}

	// Cập nhật trạng thái OTP đã xác minh
	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống"})
		return
	}
	defer tx.Rollback()

	updateOTPQuery := `UPDATE phone_verifications SET is_verified = 1 WHERE id = ?`
	if _, err := tx.ExecContext(c.Request.Context(), updateOTPQuery, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi cập nhật trạng thái OTP"})
		return
	}

	// (Tùy chọn) Cập nhật trạng thái xác minh số điện thoại cho User nếu đã đăng nhập/có thông tin user
	userID, exists := c.Get("userID")
	if exists {
		updateUserQuery := `UPDATE customers SET phone_verified_at = ? WHERE id = ?`
		_, _ = tx.ExecContext(c.Request.Context(), updateUserQuery, time.Now(), userID)
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi commit dữ liệu"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Xác thực số điện thoại thành công",
	})
}
