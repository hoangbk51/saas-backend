package services

import (
	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type PaymentProvider interface {
	// Lưu ý: Vì Interface nằm CÙNG package services với Order và Transaction
	// nên bạn KHÔNG ĐƯỢC viết models.Order, mà chỉ viết Order
	Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error)
}

// Map chứa danh sách các cổng thanh toán đã đăng ký
var paymentGateways = make(map[string]PaymentProvider)

// RegisterPaymentGateway để đăng ký các dịch vụ (ví dụ: vnpay, payos...)
func RegisterPaymentGateway(name string, provider PaymentProvider) {
	//utils.LogSQL("RegisterPaymentGateway")
	//utils.LogSQL(name)

	paymentGateways[name] = provider
}

// GetPaymentGateway lấy service tương ứng
func GetPaymentGateway(name string) (PaymentProvider, bool) {
	utils.LogSQL("GetPaymentGateway")

	utils.LogSQL(name)
	provider, ok := paymentGateways[name]
	return provider, ok
}
