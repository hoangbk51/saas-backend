package payment_gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type PaystackProvider struct {
	ClientSecret string
}

func init() {
	// Đăng ký Paystack vào hệ thống payment gateways
	services.RegisterPaymentGateway("paystack", &PaystackProvider{})
}

func (p *PaystackProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// 1. Lấy cấu hình Secret Key từ settings
	isLive := utils.GetSetting(c, "paystack_is_live", "0") == "1"
	if isLive {
		p.ClientSecret = utils.GetSetting(c, "paystack_live_secret_key", "")
	} else {
		p.ClientSecret = utils.GetSetting(c, "paystack_test_secret_key", "")
	}
	p.ClientSecret = "sk_test_664146b8a1476a175ecd248e1bd42da2ac9adc23"
	// 2. Chuẩn bị dữ liệu (Payload)
	// Lưu ý: Paystack yêu cầu amount tính bằng kobo/cent (nhân với 100)
	// Nhưng tùy thuộc vào đơn vị tiền tệ (như GHS hoặc NGN), hãy kiểm tra tài liệu Paystack
	// Ở đây tôi giữ nguyên logic nhân 100 nếu cần, hoặc khớp theo giá trị grand_total
	amount := int64(order.GrandTotal * 100)

	callbackUrl := "http://hoangk53.central.test/api/v2/paystack/callback" // Hoặc dùng hệ thống route của bạn

	metadata := map[string]interface{}{
		"custom_fields": []map[string]interface{}{
			{
				"display_name":  "Reference Code",
				"variable_name": "reference_code",
				"value":         transaction.Code,
			},
		},
	}

	payload := map[string]interface{}{
		"amount":       amount,
		"currency":     "GHS",
		"reference":    fmt.Sprintf("ref_%d", time.Now().UnixNano()),
		"callback_url": callbackUrl,
		"email":        order.Email,
		"metadata":     metadata,
	}

	// 3. Thực hiện Request
	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.paystack.co/transaction/initialize", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	// Header bắt buộc của Paystack
	req.Header.Set("Authorization", "Bearer "+p.ClientSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Paystack API Error: %v", err)
	}
	defer resp.Body.Close()

	// 4. Giải mã kết quả
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Kiểm tra trạng thái phản hồi từ Paystack
	status, _ := result["status"].(bool)
	if status {
		// Dữ liệu trả về nằm trong object "data"
		if data, ok := result["data"].(map[string]interface{}); ok {
			// Lưu reference vào transaction để đối soát sau này
			if ref, ok := data["reference"].(string); ok {
				transaction.ReferenceID = ref
			}

			return map[string]interface{}{
				"success":      true,
				"redirect_url": data["authorization_url"],
			}, nil
		}
	}

	// Trường hợp lỗi
	message, _ := result["message"].(string)
	if message == "" {
		message = "Unable to initialize Paystack transaction"
	}

	return map[string]interface{}{
		"success": false,
		"message": message,
	}, nil
}
