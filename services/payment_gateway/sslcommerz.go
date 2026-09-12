package payment_gateway

import (
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

type SSLCommerzProvider struct {
	StoreID       string
	StorePassword string
	BaseURL       string
	Currency      string
}

func init() {
	// Đăng ký cổng SSLCommerz
	services.RegisterPaymentGateway("sslcommerz", &SSLCommerzProvider{
		Currency: "BDT",
	})
}

// initConfig thiết lập cấu hình dựa trên settings
func (p *SSLCommerzProvider) initConfig(c *gin.Context) {
	isLive := utils.GetSetting(c, "sslcommerz_is_live", "0") == "1"
	mode := "sandbox"
	if isLive {
		p.StoreID = utils.GetSetting(c, "sslcommerz_live_id", "")
		p.StorePassword = utils.GetSetting(c, "sslcommerz_live_password", "")
		mode = "securepay"
	} else {
		p.StoreID = utils.GetSetting(c, "sslcommerz_test_id", "")
		p.StorePassword = utils.GetSetting(c, "sslcommerz_test_password", "")
	}

	p.StoreID = "tutao645310f10bf48"
	p.StorePassword = "tutao645310f10bf48@ssl"
	p.BaseURL = fmt.Sprintf("https://%s.sslcommerz.com", mode)
}

func (p *SSLCommerzProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	p.initConfig(c)

	// 1. Chuẩn bị dữ liệu Form Data (SSLCommerz sử dụng POST Form thay vì JSON)
	data := url.Values{}
	data.Set("store_id", p.StoreID)
	data.Set("store_passwd", p.StorePassword)
	data.Set("total_amount", fmt.Sprintf("%.2f", transaction.Amount))
	data.Set("currency", p.Currency)
	data.Set("tran_id", transaction.Code)

	// URLs Callback
	baseURL := "http://hoangk53.central.test" // Thay bằng domain thực tế của bạn
	data.Set("success_url", baseURL+"/api/v2/sslcommerz/callback")
	data.Set("fail_url", baseURL+"/sslcommerz/payment/fail")
	data.Set("cancel_url", baseURL+"/sslcommerz/payment/cancel")
	data.Set("ipn_url", baseURL+"/api/sslcommerz/ipn")

	// Thông tin khách hàng (Bắt buộc)
	data.Set("cus_name", "Customer Name")
	data.Set("cus_email", order.Email)
	data.Set("cus_add1", "Dhaka")
	data.Set("cus_city", "Dhaka")
	data.Set("cus_country", "Bangladesh")
	data.Set("cus_phone", "01711111111")
	data.Set("shipping_method", "NO")
	data.Set("product_name", "Order_"+transaction.Code)
	data.Set("product_category", "General")
	data.Set("product_profile", "general")

	// 2. Gửi Request (POST x-www-form-urlencoded)
	apiEndpoint := p.BaseURL + "/gwprocess/v4/api.php"
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.PostForm(apiEndpoint, data)
	if err != nil {
		return nil, fmt.Errorf("SSLCommerz connection error: %v", err)
	}
	defer resp.Body.Close()

	// 3. Giải mã kết quả
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// 4. Xử lý phản hồi
	if status, ok := result["status"].(string); ok && status == "SUCCESS" {
		if redirectURL, ok := result["GatewayPageURL"].(string); ok {
			return map[string]interface{}{
				"success":      true,
				"redirect_url": redirectURL,
			}, nil
		}
	}

	message := "Invalid Credentials or Connectivity Issue"
	if failedReason, ok := result["failedreason"].(string); ok {
		message = failedReason
	}

	return map[string]interface{}{
		"success": false,
		"message": message,
	}, nil
}

// PaymentCallback xử lý dữ liệu POST trả về từ SSLCommerz
func (p *SSLCommerzProvider) PaymentCallback(c *gin.Context) (map[string]interface{}, error) {
	// SSLCommerz gửi dữ liệu qua POST Form
	status := c.PostForm("status")
	tranID := c.PostForm("tran_id")
	valID := c.PostForm("val_id") // Đây là reference_id từ SSLCommerz

	if status == "VALID" {
		// Ở đây bạn thực hiện logic tương tự PHP:
		// 1. Tìm Transaction theo tranID (tran_id)
		// 2. Cập nhật reference_id = valID
		// 3. Gọi hàm success() cho transaction
		return map[string]interface{}{
			"success":      true,
			"tran_id":      tranID,
			"reference_id": valID,
		}, nil
	}

	return map[string]interface{}{
		"success": false,
		"message": "Payment status is " + status,
	}, nil
}
