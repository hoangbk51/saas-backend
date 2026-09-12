package payment_gateway

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type VNPayProvider struct {
	TmnCode    string
	HashSecret string
}

func init() {
	services.RegisterPaymentGateway("vnpay", &VNPayProvider{})
}

func (p *VNPayProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// 1. Khởi tạo cấu hình
	isLive := utils.GetSetting(c, "vnpay_is_live", "0") == "1"
	vnpURL := "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html"
	if isLive {
		p.TmnCode = utils.GetSetting(c, "vnpay_live_terminal_id", "")
		p.HashSecret = utils.GetSetting(c, "vnpay_live_secret_key", "")
		vnpURL = "https://pay.vnpayment.vn/paymentv2/vpcpay.html"
	} else {
		p.TmnCode = utils.GetSetting(c, "vnpay_test_terminal_id", "")
		p.HashSecret = utils.GetSetting(c, "vnpay_test_secret_key", "")
	}

	// 2. Chuẩn bị dữ liệu
	now := time.Now()
	expire := now.Add(48 * time.Hour) // +2 days
	amount := int64(order.GrandTotal * 100)

	inputData := url.Values{}
	inputData.Set("vnp_Version", "2.1.0")
	inputData.Set("vnp_Command", "pay")
	inputData.Set("vnp_TmnCode", p.TmnCode)
	inputData.Set("vnp_Amount", fmt.Sprintf("%d", amount))
	inputData.Set("vnp_CurrCode", "VND")
	inputData.Set("vnp_TxnRef", transaction.Code)
	inputData.Set("vnp_OrderInfo", "Thanh toan GD:"+transaction.Code)
	inputData.Set("vnp_OrderType", "other")
	inputData.Set("vnp_Locale", "vn")
	inputData.Set("vnp_ReturnUrl", "https://yourdomain.com/vnpay/callback")
	inputData.Set("vnp_IpAddr", c.ClientIP())
	inputData.Set("vnp_CreateDate", now.Format("20060102150405"))
	inputData.Set("vnp_ExpireDate", expire.Format("20060102150405"))

	// 3. Sắp xếp tham số theo alphabet (Ksort)
	keys := make([]string, 0, len(inputData))
	for k := range inputData {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 4. Tạo chuỗi HashData và Query
	var rawData strings.Builder
	var query strings.Builder
	for i, k := range keys {
		val := inputData.Get(k)
		if i > 0 {
			rawData.WriteString("&")
		}
		rawData.WriteString(url.QueryEscape(k) + "=" + url.QueryEscape(val))
		query.WriteString(url.QueryEscape(k) + "=" + url.QueryEscape(val) + "&")
	}

	// 5. Tính toán mã băm SHA512
	h := hmac.New(sha512.New, []byte(p.HashSecret))
	h.Write([]byte(rawData.String()))
	secureHash := hex.EncodeToString(h.Sum(nil))

	fullURL := vnpURL + "?" + query.String() + "vnp_SecureHash=" + secureHash

	return map[string]interface{}{
		"success":      true,
		"redirect_url": fullURL,
	}, nil
}

// SuccessCheckout (IPN/Callback handler)
func (p *VNPayProvider) PaymentCallback(c *gin.Context) (map[string]interface{}, error) {
	// Khởi tạo Secret để verify
	isLive := utils.GetSetting(c, "vnpay_is_live", "0") == "1"
	if isLive {
		p.HashSecret = utils.GetSetting(c, "vnpay_live_secret_key", "")
	} else {
		p.HashSecret = utils.GetSetting(c, "vnpay_test_secret_key", "")
	}

	queryParams := c.Request.URL.Query()
	vnpSecureHash := c.Query("vnp_SecureHash")

	// 1. Thu thập các tham số vnp_ và sắp xếp (loại bỏ SecureHash)
	var keys []string
	for k := range queryParams {
		if strings.HasPrefix(k, "vnp_") && k != "vnp_SecureHash" && k != "vnp_SecureHashType" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	// 2. Build lại rawData để verify signature
	var rawData strings.Builder
	for i, k := range keys {
		if i > 0 {
			rawData.WriteString("&")
		}
		rawData.WriteString(url.QueryEscape(k) + "=" + url.QueryEscape(queryParams.Get(k)))
	}

	// 3. Kiểm tra chữ ký
	h := hmac.New(sha512.New, []byte(p.HashSecret))
	h.Write([]byte(rawData.String()))
	checkHash := hex.EncodeToString(h.Sum(nil))

	if checkHash != vnpSecureHash {
		return map[string]interface{}{"RspCode": "97", "Message": "Invalid Signature"}, nil
	}

	// 4. Kiểm tra logic nghiệp vụ (Số tiền, trạng thái đơn hàng...)
	// Tương tự PHP: Lấy transaction bằng vnp_TxnRef, so khớp vnp_Amount...

	vnpResponseCode := c.Query("vnp_ResponseCode")
	if vnpResponseCode == "00" {
		// Thanh toán thành công
		return map[string]interface{}{"RspCode": "00", "Message": "Confirm Success"}, nil
	}

	return map[string]interface{}{"RspCode": "01", "Message": "Payment Failed"}, nil
}
