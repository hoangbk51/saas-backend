package payment_gateway

import (
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type InstamojoProvider struct {
	APIKey    string
	AuthToken string
	EndPoint  string
}

func init() {
	// Đăng ký gateway vào hệ thống
	services.RegisterPaymentGateway("instamojo", &InstamojoProvider{})
}

func (p *InstamojoProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// 1. Khởi tạo cấu hình từ Settings
	isLive := utils.GetSetting(c, "instamojo_is_live", "") == "1"
	if isLive {
		p.APIKey = utils.GetSetting(c, "instamojo_live_api_key", "")
		p.AuthToken = utils.GetSetting(c, "instamojo_live_auth_token", "")
		p.EndPoint = "https://www.instamojo.com/api/1.1/payment-requests/"
	} else {
		p.APIKey = utils.GetSetting(c, "instamojo_test_api_key", "")
		p.AuthToken = utils.GetSetting(c, "instamojo_test_auth_token", "")
		p.EndPoint = "https://test.instamojo.com/api/1.1/payment-requests/"
	}

	// Kiểm tra số điện thoại (giống logic PHP của bạn)
	if order.CustomerPhone == "" {
		return map[string]interface{}{
			"success": false,
			"message": "Please add phone number to your profile",
		}, nil
	}

	// 2. Chuẩn bị Form Data (Instamojo API v1.1 thường dùng x-www-form-urlencoded)
	data := url.Values{}
	data.Set("purpose", fmt.Sprintf("payment for order #%d", order.ID))
	data.Set("amount", fmt.Sprintf("%.2f", order.GrandTotal)) // round grand_total
	data.Set("send_email", "false")
	data.Set("email", order.Email)
	data.Set("phone", order.CustomerPhone)
	data.Set("redirect_url", "https://yourdomain.com/instamojo/payment/pay-success")

	// 3. Thực hiện Request
	req, err := http.NewRequest("POST", p.EndPoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// Header bắt buộc của Instamojo
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Api-Key", p.APIKey)
	req.Header.Set("X-Auth-Token", p.AuthToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Instamojo Connection Error: %v", err)
	}
	defer resp.Body.Close()

	// 4. Giải mã kết quả
	var result struct {
		Success        bool `json:"success"`
		PaymentRequest struct {
			LongURL string `json:"longurl"`
		} `json:"payment_request"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Success {
		return map[string]interface{}{
			"success":      true,
			"redirect_url": result.PaymentRequest.LongURL,
			"message":      "URL generated",
		}, nil
	}

	return map[string]interface{}{
		"success": false,
		"message": result.Message,
	}, nil
}
