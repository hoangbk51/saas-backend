package payment_gateway

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

type PayOSProvider struct {
	ClientID    string
	APIKey      string
	ChecksumKey string
}

func init() {
	// Đăng ký cổng thanh toán PayOS
	services.RegisterPaymentGateway("payos", &PayOSProvider{})
}

func (p *PayOSProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// 1. Lấy cấu hình từ Settings
	isLive := utils.GetSetting(c, "payos_is_live", "0") == "1"
	if isLive {
		p.ClientID = utils.GetSetting(c, "payos_live_client_id", "")
		p.APIKey = utils.GetSetting(c, "payos_live_api_key", "")
		p.ChecksumKey = utils.GetSetting(c, "payos_live_checksum_key", "")
	} else {
		p.ClientID = utils.GetSetting(c, "payos_test_client_id", "")
		p.APIKey = utils.GetSetting(c, "payos_test_api_key", "")
		p.ChecksumKey = utils.GetSetting(c, "payos_test_checksum_key", "")
	}

	// 2. Chuẩn bị dữ liệu thanh toán
	// Lưu ý: orderCode của PayOS phải là số (int64)
	orderCode := int64(order.ID)
	amount := int64(order.GrandTotal)
	description := transaction.Code // Tối đa 25 ký tự
	if len(description) > 25 {
		description = description[:25]
	}

	returnUrl := "https://yourdomain.com/payos/payment/return"
	cancelUrl := "https://yourdomain.com/payos/payment/cancel"

	// 3. Tạo Signature cho dữ liệu (PayOS yêu cầu checksum để bảo mật)
	// Chuỗi data để hash: amount, cancelUrl, description, orderCode, returnUrl
	params := map[string]interface{}{
		"amount":      amount,
		"cancelUrl":   cancelUrl,
		"description": description,
		"orderCode":   orderCode,
		"returnUrl":   returnUrl,
	}
	signature := p.createSignature(params, p.ChecksumKey)

	// 4. Tạo Payload gửi API
	payload := map[string]interface{}{
		"orderCode":   orderCode,
		"amount":      amount,
		"description": description,
		"cancelUrl":   cancelUrl,
		"returnUrl":   returnUrl,
		"signature":   signature,
	}

	// 5. Thực hiện Request đến PayOS
	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api-merchant.payos.vn/v2/payment-requests", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	// PayOS yêu cầu các Header đặc thù
	req.Header.Set("x-client-id", p.ClientID)
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PayOS API Error: %v", err)
	}
	defer resp.Body.Close()

	// 6. Giải mã kết quả
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// PayOS trả về code "00" nếu thành công
	if code, ok := result["code"].(string); ok && code == "00" {
		if data, ok := result["data"].(map[string]interface{}); ok {
			return map[string]interface{}{
				"success":      true,
				"redirect_url": data["checkoutUrl"],
			}, nil
		}
	}

	return map[string]interface{}{
		"success": false,
		"message": result["desc"],
	}, nil
}

// createSignature tạo chữ ký HMAC SHA256 dựa trên alphabet order của key
func (p *PayOSProvider) createSignature(data map[string]interface{}, checksumKey string) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var rawData string
	for _, k := range keys {
		if rawData != "" {
			rawData += "&"
		}
		rawData += fmt.Sprintf("%s=%v", k, data[k])
	}

	h := hmac.New(sha256.New, []byte(checksumKey))
	h.Write([]byte(rawData))
	return hex.EncodeToString(h.Sum(nil))
}
