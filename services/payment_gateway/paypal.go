package payment_gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// PayPalProvider thực hiện interface PaymentProvider
type PayPalProvider struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
}

func init() {
	utils.LogToFile("init paypal")

	//utils.LogSQL("inint paypal")
	// Tự động đăng ký vào hệ thống gateways
	services.RegisterPaymentGateway("paypal", &PayPalProvider{})

}

// Struct để parse kết quả từ PayPal API
type paypalOrderResponse struct {
	ID    string `json:"id"`
	Links []struct {
		Href   string `json:"href"`
		Rel    string `json:"rel"`
		Method string `json:"method"`
	} `json:"links"`
}

func (p *PayPalProvider) Pay(c *gin.Context, order *models.Order, tx *models.Transaction) (map[string]interface{}, error) {
	// 1. Khởi tạo cấu hình

	p.ClientID, _ = utils.GetPluginSetting(c, "paypal", "paypal_test_client_id")
	p.ClientSecret, _ = utils.GetPluginSetting(c, "paypal", "paypal_test_client_secret") // "EOHTvu5jv4hNwws1whRjHfGrw6yVkWU0T24dcOfgC4rRp9NjBLnXbgvgNNhubMHxUibgzfyL25UU6CJU"
	p.BaseURL = "https://api-m.sandbox.paypal.com"                                       // Hoặc production -todo
	isLiveStr, _ := utils.GetPluginSetting(c, "paypal", "paypal_is_live")
	isLive, _ := strconv.Atoi(isLiveStr)
	utils.LogToFile(p.ClientID)
	utils.LogToFile(p.ClientSecret)
	if isLive == 0 {
		p.ClientID, _ = utils.GetPluginSetting(c, "paypal", "paypal_test_client_id")
		p.ClientSecret, _ = utils.GetPluginSetting(c, "paypal", "paypal_test_client_secret")
	} else {
		p.ClientID, _ = utils.GetPluginSetting(c, "paypal", "paypal_live_client_id")
		p.ClientSecret, _ = utils.GetPluginSetting(c, "paypal", "paypal_live_client_secret")
	}
	// 2. Lấy Access Token (PayPal yêu cầu OAuth2)
	accessToken, err := p.getAccessToken()
	if err != nil {
		return nil, err
	}

	var baseURL = utils.GetCurrentDomain(c)

	// 3. Chuẩn bị Body Request (Tương đương OrdersCreateRequest bên PHP)
	amountStr := fmt.Sprintf("%.2f", order.GrandTotal)
	body := map[string]interface{}{
		"intent": "CAPTURE",
		"purchase_units": []map[string]interface{}{
			{
				"reference_id": tx.Code,
				"amount": map[string]string{
					"currency_code": "USD",
					"value":         amountStr,
				},
			},
		},
		"application_context": map[string]string{
			"cancel_url": fmt.Sprintf("%s/paypal/cancel", baseURL),
			"return_url": fmt.Sprintf("%s/api/v2/paypal/callback", baseURL),
		},
	}

	jsonBody, _ := json.Marshal(body)

	// 4. Gọi PayPal API
	req, _ := http.NewRequest("POST", p.BaseURL+"/v2/checkout/orders", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 5. Xử lý kết quả trả về
	var res paypalOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	// Tìm link redirect cho khách hàng (thường là link có rel="approve")
	redirectURL := ""
	for _, link := range res.Links {
		if link.Rel == "approve" {
			redirectURL = link.Href
			break
		}
	}

	return map[string]interface{}{
		"success":      true,
		"redirect_url": redirectURL,
		"order_id":     res.ID, // PayPal Order ID để capture sau này
	}, nil
}

// Helper lấy Access Token
func (p *PayPalProvider) getAccessToken() (string, error) {
	req, _ := http.NewRequest("POST", p.BaseURL+"/v1/oauth2/token", bytes.NewBufferString("grant_type=client_credentials"))
	req.SetBasicAuth(p.ClientID, p.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	return data["access_token"].(string), nil
}

/*
func getPayPalClient() (*paypal.Client, error) {

		clientID := "AfZxSTWd0odjNSlXGUEizt9-NFrzSILX_LmXzPMWUS7uAxkSG91oOpY5oNuvnJMVGUfgqZjNJcOt9kPb"
		secret := "EOHTvu5jv4hNwws1whRjHfGrw6yVkWU0T24dcOfgC4rRp9NjBLnXbgvgNNhubMHxUibgzfyL25UU6CJU"
		base := "https://api-m.sandbox.paypal.com" // Hoặc production -todo

		client, err := paypal.NewClient(clientID, secret, base)
		if err != nil {
			return nil, err
		}
		return client, nil
	}

	func PayPalCallbackHandler(c *gin.Context) {
		// 1. Lấy token (Order ID) từ URL query
		orderID := c.Query("token") // PayPal trả về ?token=XXXX
		if orderID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing token"})
			return
		}

		client, err := getPayPalClient()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "PayPal client error"})
			return
		}

		// 2. Capture Order (Tương đương $client->execute($ordersCaptureRequest))
		// PayPal yêu cầu GetAccessToken trước khi thực hiện capture
		_, err = client.GetAccessToken(context.Background())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Auth error"})
			return
		}

		capture, err := client.CaptureOrder(context.Background(), orderID, paypal.CaptureOrderRequest{})
		if err != nil {
			log.Printf("Capture Error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Capture failed"})
			return
		}

		// 3. Xử lý Logic Database (Tương đương logic Eloquent của Laravel)
		if capture.Status == "COMPLETED" {
			// Lấy reference_id từ purchase_units
			if len(capture.PurchaseUnits) > 0 {
				referenceID := capture.PurchaseUnits[0].ReferenceID

				// Cập nhật Database
				db, _ := utils.GetDBFromContext(c)

				// Cập nhật trạng thái Transaction
				// STATUS = 1 (Thành công)
				query := `UPDATE transactions SET status = 1, updated_at = NOW() WHERE code = ?`
				_, err := db.Exec(query, referenceID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "DB Update failed"})
					return
				}

				// Lấy OrderID để chuyển hướng hoặc gọi CheckoutDone
				var shopOrderID uint64
				db.QueryRow("SELECT order_id FROM transactions WHERE code = ?", referenceID).Scan(&shopOrderID)

				// 4. Redirect hoặc Response thành công (Giống checkout_done)
				c.JSON(http.StatusOK, gin.H{
					"status":   "success",
					"order_id": shopOrderID,
					"message":  "Payment captured successfully",
				})
				return
			}
		}

		c.JSON(http.StatusPaymentRequired, gin.H{"status": "failed"})
	}
*/
type paypalCaptureResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	PurchaseUnits []struct {
		ReferenceID string `json:"reference_id"`
	} `json:"purchase_units"`
}

func (p *PayPalProvider) CapturePayment(orderID string) (*paypalCaptureResponse, error) {
	p.BaseURL = "https://api-m.sandbox.paypal.com" // Hoặc production -todo

	// 1. Lấy Access Token
	accessToken, err := p.getAccessToken()
	if err != nil {
		return nil, err
	}

	// 2. Gọi PayPal API Capture
	url := fmt.Sprintf("%s/v2/checkout/orders/%s/capture", p.BaseURL, orderID)
	req, _ := http.NewRequest("POST", url, nil) // Capture không cần body
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 3. Parse kết quả
	var result paypalCaptureResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
