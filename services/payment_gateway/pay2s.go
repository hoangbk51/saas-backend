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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Pay2SProvider struct {
	PartnerCode string
	SecretKey   string
	AccessKey   string
	EndPoint    string
}

func init() {
	// Tự động đăng ký vào hệ thống gateways
	services.RegisterPaymentGateway("pay2s", &Pay2SProvider{})
}

func (p *Pay2SProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// 1. Lấy cấu hình sử dụng utils.GetSetting
	isLive := utils.GetSetting(c, "pay2s_is_live", "0") == "1"
	if isLive {
		p.PartnerCode = utils.GetSetting(c, "pay2s_live_partner_code", "")
		p.SecretKey = utils.GetSetting(c, "pay2s_live_secret_key", "")
		p.AccessKey = utils.GetSetting(c, "pay2s_live_access_key", "")
		p.EndPoint = "https://payment.pay2s.vn/v1/gateway/api"
	} else {
		p.PartnerCode = utils.GetSetting(c, "pay2s_test_partner_code", "")
		p.SecretKey = utils.GetSetting(c, "pay2s_test_secret_key", "")
		p.AccessKey = utils.GetSetting(c, "pay2s_test_access_key", "")
		p.EndPoint = "https://sandbox-payment.pay2s.vn/v1/gateway/api"
	}

	// 2. Chuẩn bị dữ liệu bổ trợ
	amount := int64(order.GrandTotal) // Giả định GrandTotal là float/int cần convert sang int64
	requestId := fmt.Sprintf("%d", time.Now().Unix())
	orderId := fmt.Sprintf("%v", order.ID)
	orderInfo := "ThanhToanDonHang" + orderId // Pay2S giới hạn 10-32 ký tự, không dấu

	redirectUrl := fmt.Sprintf("https://yourdomain.com/pay2s/success?code=%s", transaction.Code)
	ipnUrl := "https://yourdomain.com/api/pay2s/webhook"
	requestType := "pay2s"

	// Xử lý Bank Accounts (Logic từ PHP của bạn)
	bankAccountsStr := "345875|ACB" // Có thể lấy từ utils.GetSetting nếu cần
	var bankList []map[string]string
	lines := strings.Split(bankAccountsStr, "\n")
	for _, line := range lines {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) == 2 {
			bankList = append(bankList, map[string]string{
				"account_number": strings.TrimSpace(parts[0]),
				"bank_id":        strings.TrimSpace(parts[1]),
			})
		}
	}

	// 3. Tạo chữ ký HMAC SHA256
	// Lưu ý: bankAccounts=Array là quy định của Pay2S khi tạo chuỗi hash
	rawHash := fmt.Sprintf(
		"accessKey=%s&amount=%d&bankAccounts=Array&ipnUrl=%s&orderId=%s&orderInfo=%s&partnerCode=%s&redirectUrl=%s&requestId=%s&requestType=%s",
		p.AccessKey, amount, ipnUrl, orderId, orderInfo, p.PartnerCode, redirectUrl, requestId, requestType,
	)

	signature := p.computeHmacSha256(rawHash, p.SecretKey)

	// 4. Chuẩn bị Payload
	payload := map[string]interface{}{
		"accessKey":    p.AccessKey,
		"partnerCode":  p.PartnerCode,
		"partnerName":  "Merchant Name",
		"requestId":    requestId,
		"amount":       amount,
		"orderId":      transaction.Code,
		"orderInfo":    orderInfo,
		"orderType":    requestType,
		"bankAccounts": bankList,
		"redirectUrl":  redirectUrl,
		"ipnUrl":       ipnUrl,
		"requestType":  requestType,
		"signature":    signature,
	}

	// 5. Thực hiện Request
	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", p.EndPoint+"/create", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Pay2S API Error: %v", err)
	}
	defer resp.Body.Close()

	// 6. Giải mã kết quả
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Kiểm tra kết quả từ Pay2S (resultCode = 0 là thành công)
	resultCode, _ := result["resultCode"].(float64)
	if resp.StatusCode == 200 && resultCode == 0 {
		// Pay2S thường trả về danh sách QR
		return map[string]interface{}{
			"success":      true,
			"qr_code_list": result["qrList"],
		}, nil
	}

	message, _ := result["message"].(string)
	if message == "" {
		message = "Không thể kết nối Pay2S"
	}

	return map[string]interface{}{
		"success": false,
		"message": message,
	}, nil
}

// Hàm tính HMAC SHA256
func (p *Pay2SProvider) computeHmacSha256(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
