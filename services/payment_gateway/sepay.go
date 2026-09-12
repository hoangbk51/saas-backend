package payment_gateway

import (
	"fmt"
	"go-saas/models"
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

// SepayProvider thực hiện interface PaymentProvider
type SepayProvider struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
}

func init() {
	//utils.LogSQL("inint sepay")
	// Tự động đăng ký vào hệ thống gateways
	services.RegisterPaymentGateway("sepay", &SepayProvider{})

}

func (p *SepayProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {
	// Lấy mã tham chiếu từ transaction (tương đương $transaction->code)
	referenceCode := transaction.Code

	// Tạo link QR Code (Bạn có thể thay các thông số acc, bank, amount bằng dữ liệu thực tế từ order)
	// Ở đây tôi dùng fmt.Sprintf để nối chuỗi linh hoạt
	qrURL := fmt.Sprintf(
		"https://qr.sepay.vn/img?acc=101005653201&bank=VietinBank&amount=%d&des=SEVQR %s",
		2000, // Giả sử order.Total là số tiền  int(order.GrandTotal)
		referenceCode,
	)

	// Trả về map tương đương với array trong PHP
	// 'qr_code_list' là một slice chứa các map
	qrCodeList := []map[string]string{
		{
			"qrUrl": qrURL,
			"name":  "sepay",
		},
	}

	return map[string]interface{}{
		"success":      true,
		"qr_code_list": qrCodeList,
	}, nil
}
