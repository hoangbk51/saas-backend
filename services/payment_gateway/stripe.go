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

type StripeProvider struct {
	SecretKey string
}

func init() {
	// Đăng ký Stripe vào hệ thống
	services.RegisterPaymentGateway("stripe", &StripeProvider{})
}

func (p *StripeProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// 1. Lấy cấu hình Secret Key
	isLive := utils.GetSetting(c, "stripe_is_live", "0") == "1"
	if isLive {
		p.SecretKey = utils.GetSetting(c, "stripe_live_secret_key", "")
	} else {
		p.SecretKey = utils.GetSetting(c, "stripe_test_secret_key", "")
	}
	p.SecretKey = ""
	// 2. Chuẩn bị dữ liệu Form Data (Stripe API sử dụng x-www-form-urlencoded)
	// Lưu ý: Stripe yêu cầu số tiền tính bằng đơn vị nhỏ nhất (cent/eurocent)
	amount := int64(order.GrandTotal * 100)

	data := url.Values{}
	data.Set("mode", "payment")
	data.Set("success_url", "http://hoangk53.central.test/api/v2/stripe/success?session_id={CHECKOUT_SESSION_ID}")
	data.Set("cancel_url", "https://yourdomain.com/stripe/cancel")

	// Cấu hình Item
	data.Set("line_items[0][price_data][currency]", "eur")
	data.Set("line_items[0][price_data][unit_amount]", fmt.Sprintf("%d", amount))
	data.Set("line_items[0][price_data][product_data][name]", fmt.Sprintf("Thanh toán đơn hàng #%d", order.ID))
	data.Set("line_items[0][quantity]", "1")

	// Thu thập địa chỉ thanh toán (Bắt buộc cho Klarna/Afterpay nếu bạn bật sau này)
	data.Set("billing_address_collection", "required")

	// Metadata
	data.Set("metadata[order_id]", fmt.Sprintf("%d", order.ID))
	data.Set("metadata[reference_code]", transaction.Code)

	// 3. Thực hiện Request đến Stripe API
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("POST", "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	// Stripe Authentication
	req.Header.Set("Authorization", "Bearer "+p.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Stripe API Connection Error: %v", err)
	}
	defer resp.Body.Close()

	// 4. Giải mã kết quả
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Kiểm tra nếu có lỗi từ Stripe (ví dụ Key sai)
	if stripeErr, ok := result["error"].(map[string]interface{}); ok {
		return map[string]interface{}{
			"success": false,
			"message": stripeErr["message"],
		}, nil
	}

	// Trả về URL thanh toán
	if checkoutURL, ok := result["url"].(string); ok {
		return map[string]interface{}{
			"success":      true,
			"redirect_url": checkoutURL,
		}, nil
	}

	return map[string]interface{}{
		"success": false,
		"message": "Could not create Stripe session",
	}, nil
}
