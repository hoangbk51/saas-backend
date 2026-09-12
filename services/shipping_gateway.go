package services

import (
	"go-saas/models"

	"go-saas/utils"
)

// ShippingProvider định nghĩa các phương thức mà mọi đơn vị vận chuyển phải có
type ShippingProvider interface {
	// Trả về số tiền phí ship và lỗi nếu có
	CalculateFee(order *models.Cart) (float64, error)
	GetName() string // Để hiển thị "Giao Hàng Nhanh", "Viettel Post", v.v.
	GetServices(cart *models.Cart) ([]models.ShippingService, error)
}

// Map chứa danh sách các đơn vị vận chuyển đã đăng ký
var shippingGateways = make(map[string]ShippingProvider)

// RegisterShippingGateway đăng ký đơn vị vận chuyển (ví dụ: ghn, ghtk, nội bộ...)
func RegisterShippingGateway(name string, provider ShippingProvider) {
	utils.LogToFile("RegisterShippingGateway: " + name)
	shippingGateways[name] = provider
}

// GetShippingGateway lấy provider tương ứng theo code
func GetShippingGateway(name string) (ShippingProvider, bool) {
	provider, ok := shippingGateways[name]
	return provider, ok
}

func GetAllShippingProviders() map[string]ShippingProvider {
	return shippingGateways
}
