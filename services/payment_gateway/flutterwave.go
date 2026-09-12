package payment_gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils" // Giả sử bạn có hàm lấy config/setting ở đây
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type FlutterwaveProvider struct {
	SecretKey string
	EndPoint  string
}

// Struct để map dữ liệu gửi lên Flutterwave API
type FlutterwaveRequest struct {
	PaymentOptions string                 `json:"payment_options"`
	Amount         float64                `json:"amount"`
	Email          string                 `json:"email"`
	TxRef          string                 `json:"tx_ref"`
	Currency       string                 `json:"currency"`
	RedirectURL    string                 `json:"redirect_url"`
	Customer       map[string]string      `json:"customer"`
	Customizations map[string]interface{} `json:"customizations"`
}

func init() {
	// Tự động đăng ký gateway
	services.RegisterPaymentGateway("flutterwave", &FlutterwaveProvider{
		EndPoint: "https://api.flutterwave.com/v3",
	})
}

func (p *FlutterwaveProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// 1. Khởi tạo cấu hình (Tương đương hàm init() trong PHP)
	// Lưu ý: Bạn cần hàm lấy setting từ DB hoặc file config
	isLive := utils.GetSetting(c, "flutterware_is_live", "0") == "1"
	if isLive {
		p.SecretKey = utils.GetSetting(c, "flutterware_live_secret_key", "")
	} else {
		p.SecretKey = utils.GetSetting(c, "flutterware_test_secret_key", "")
	}
	utils.LogSQL(order.Email)
	p.SecretKey = ""
	// 2. Chuẩn bị dữ liệu Payload
	data := FlutterwaveRequest{
		PaymentOptions: "card,banktransfer",
		Amount:         3.0, // Theo yêu cầu fix cứng 3 USD của bạn
		Email:          order.Email,
		TxRef:          transaction.Code,
		Currency:       "USD",
		RedirectURL:    "https://hoangk53.central.test/api/v2/flutterwave/callback", // Route callback của bạn
		Customer: map[string]string{
			"email":        order.Email,
			"phone_number": order.CustomerPhone,
			"name":         fmt.Sprintf("%s %s", order.BillingFirstName, order.BillingLastName),
		},
		Customizations: map[string]interface{}{
			"title":       "Payment",
			"description": "",
		},
	}

	// 3. Thực hiện HTTP POST request
	jsonData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", p.EndPoint+"/payments", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.SecretKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 4. Giải mã kết quả trả về
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("HTTP Status: %d\n", resp.StatusCode)
	fmt.Printf("API Response: %+v\n", result)
	// Kiểm tra trạng thái thành công
	if resp.StatusCode == http.StatusOK && result["status"] == "success" {
		// Lấy link từ result["data"]["link"]
		if dataField, ok := result["data"].(map[string]interface{}); ok {
			return map[string]interface{}{
				"success":      true,
				"redirect_url": dataField["link"],
				"message":      "Url generated",
			}, nil
		}
	}

	return map[string]interface{}{
		"success": false,
		"message": "Could not find redirect url",
	}, nil
}
