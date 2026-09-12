package shipping

import (
	"go-saas/models"

	"github.com/gin-gonic/gin"
)

type ShippingProvider interface {
	CalculateFee(c *gin.Context, cart *models.Cart) (float64, error)
	GetServices(c *gin.Context, cart *models.Cart) ([]models.ShippingService, error)
	GetName() string
	CreateOrder(c *gin.Context, order *models.Order, note string) (trackingCode string, fee float64, err error)
	CancelShipment(c *gin.Context, trackingId string) error
}

var gateways = make(map[string]ShippingProvider)

func Register(name string, provider ShippingProvider) {
	gateways[name] = provider
}

func Get(name string) (ShippingProvider, bool) {
	provider, ok := gateways[name]
	return provider, ok
}
