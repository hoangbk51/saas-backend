package payment_gateway

import (
	"fmt"
	"go-saas/models"
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

type AuthorizeNetProvider struct {
	Key    string
	Secret string
}

func init() {
	// Đăng ký Razorpay vào hệ thống gateways
	services.RegisterPaymentGateway("authorize_net", &AuthorizeNetProvider{})
}

func (p *AuthorizeNetProvider) Pay(c *gin.Context, order *models.Order, transaction *models.Transaction) (map[string]interface{}, error) {

	// Trong logic PHP của bạn, route('razorpay.payment') là nơi chứa giao diện script của Razorpay
	redirectURL := fmt.Sprintf("/partner/payment/authorizenet/%s", transaction.Code)
	return map[string]interface{}{
		"success":      true,
		"redirect_url": redirectURL,
	}, nil
}
