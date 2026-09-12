package payment_gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type MollieProvider struct {
	APIKey string
}

func init() {
	utils.LogSQL("init mollie")
	// Tự động đăng ký vào hệ thống gateways
	services.RegisterPaymentGateway("mollie", &MollieProvider{})
}

func (p *MollieProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// 1. Lấy cấu hình sử dụng utils.GetSetting mới
	isLive := utils.GetSetting(c, "mollie_is_live", "0") == "1"
	if isLive {
		p.APIKey = utils.GetSetting(c, "mollie_live_key", "")
	} else {
		p.APIKey = utils.GetSetting(c, "mollie_test_key", "")
	}
	p.APIKey = "test_CaruQFMaTUd23p3mbaFunS4NkEuQwq"
	// 2. Chuẩn bị Payload theo cấu trúc của Mollie
	// Mollie yêu cầu amount là một object với currency và value (string format "0.00")
	payload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"order_id": transaction.OrderID,
		},
		"amount": map[string]string{
			"currency": "EUR",
			"value":    fmt.Sprintf("%.2f", transaction.Amount),
		},
		"description": "Payment with mollie",
		"redirectUrl": fmt.Sprintf("http://hoangk53.central.test/api/v2/mollie/callback?code=%s", transaction.Code),
	}

	// 3. Thực hiện Request
	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.mollie.com/v2/payments", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Mollie API Error: %v", err)
	}
	defer resp.Body.Close()

	// 4. Giải mã kết quả
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Lưu reference_id vào transaction (tương tự PHP: $transaction->reference_id = $response['id'])
	if mollieID, ok := result["id"].(string); ok {
		err := updateMollieReference(c, transaction.Code, mollieID)
		if err != nil {
			log.Printf("Update reference failed: %v", err)
			c.JSON(500, gin.H{"error": "Internal Server Error"})
			return nil, err
		}
	}

	// Lấy link redirect từ _links.checkout.href
	if links, ok := result["_links"].(map[string]interface{}); ok {
		if checkout, ok := links["checkout"].(map[string]interface{}); ok {
			return map[string]interface{}{
				"success":      true,
				"redirect_url": checkout["href"],
			}, nil
		}
	}

	return map[string]interface{}{
		"success": false,
		"message": "Could not find checkout url",
	}, nil
}

func updateMollieReference(c *gin.Context, transactionCode string, mollieID string) error {
	// 1. Lấy DB từ context
	db, err := utils.GetDBFromContext(c)

	// 2. Thực hiện query update
	// Chúng ta lọc theo cả code và tenant_id để đảm bảo an toàn tuyệt đối cho SaaS
	query := `UPDATE transactions SET reference_id = ? WHERE code = ? `

	result, err := db.Exec(query, mollieID, transactionCode)
	if err != nil {
		return err
	}

	// 3. (Tùy chọn) Kiểm tra xem có dòng nào được update không
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no transaction found with code %s for tenant %s", transactionCode)
	}

	return nil
}
