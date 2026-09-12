package payment_gateway

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type RazorpayProvider struct {
	Key    string
	Secret string
}

func init() {
	// Đăng ký Razorpay vào hệ thống gateways
	services.RegisterPaymentGateway("razorpay", &RazorpayProvider{})
}

// Pay trong Razorpay thường chỉ trả về một route nội bộ để hiển thị form Checkout của Razorpay
func (p *RazorpayProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// Trong logic PHP của bạn, route('razorpay.payment') là nơi chứa giao diện script của Razorpay
	redirectURL := fmt.Sprintf("/partner/payment/razorpay/%s", transaction.Code)

	return map[string]interface{}{
		"success":      true,
		"redirect_url": redirectURL,
	}, nil
}

// PaymentCallback xử lý xác thực sau khi người dùng thực hiện thanh toán trên giao diện
func (p *RazorpayProvider) PaymentCallback(c *gin.Context) (map[string]interface{}, error) {
	// 1. Khởi tạo cấu hình
	isLive := utils.GetSetting(c, "razorpay_is_live", "0") == "1"
	if isLive {
		p.Key = utils.GetSetting(c, "razorpay_live_key", "")
		p.Secret = utils.GetSetting(c, "razorpay_live_secret", "")
	} else {
		p.Key = utils.GetSetting(c, "razorpay_test_key", "")
		p.Secret = utils.GetSetting(c, "razorpay_test_secret", "")
	}
	p.Key = "rzp_test_PEMGZ0g2W0kpeq"
	p.Secret = "jSZffOIDHKys8LNgyfRHVxqE"
	// 2. Lấy thông tin từ request (Razorpay gửi payment_id về sau khi thanh toán thành công ở client)
	paymentID := c.Query("razorpay_payment_id")
	//code := c.Param("code") // Giả sử code được truyền qua URL param

	if paymentID == "" {
		return nil, fmt.Errorf("missing razorpay_payment_id")
	}

	// 3. Gọi API Razorpay để kiểm tra trạng thái thanh toán
	auth := base64.StdEncoding.EncodeToString([]byte(p.Key + ":" + p.Secret))
	url := fmt.Sprintf("https://api.razorpay.com/v1/payments/%s", paymentID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Razorpay API Error: %v", err)
	}
	defer resp.Body.Close()

	// 4. Giải mã kết quả
	var payment map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payment); err != nil {
		return nil, err
	}

	// 5. Kiểm tra trạng thái (authorized hoặc captured là thành công tùy cấu hình)
	status, _ := payment["status"].(string)
	if status == "authorized" || status == "captured" {
		// Logic cập nhật Transaction tương tự PHP
		// services.UpdateTransactionSuccess(code, paymentID)

		return map[string]interface{}{
			"success":      true,
			"reference_id": payment["id"],
			"message":      "Payment successful",
		}, nil
	}

	return map[string]interface{}{
		"success": false,
		"message": "Payment failed or not authorized",
	}, nil
}
