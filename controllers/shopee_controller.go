package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go-saas/services"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

// Helper lấy base URL Shopee Mock
func getShopeeMockBase(c *gin.Context) string {
	return utils.GetSetting(c, "shopee_mock_url", "http://localhost:3003")
}

// 1. Get Connect URL
func GetShopeeConnectURL(c *gin.Context) {
	partnerID := utils.GetSetting(c, "shopee_partner_id", "mock_partner_123")
	redirectURL := utils.GetSetting(c, "shopee_redirect_url", "http://localhost:8080/api/v1/shopee/callback")
	authURL := fmt.Sprintf("%s/api/v2/shopee/auth_partner?partner_id=%s&redirect=%s", getShopeeMockBase(c), partnerID, redirectURL)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "auth_url": authURL})
}
func proxyMockResponse(c *gin.Context, method string, mockURL string, body interface{}) {
	headers := map[string]string{
		"Authorization": c.GetHeader("Authorization"),
	}

	respBytes, err := utils.RequestMock(c, method, mockURL, headers, body)
	if err != nil && len(respBytes) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi parse dữ liệu từ Mock Server"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// --- SHOPEE ORDER APIS ---

func FetchShopeeOrderList(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/order/get_order_list?partner_id=%s&shop_id=%s",
		getShopeeMockBase(c), c.Query("partner_id"), c.Query("shop_id"))
	proxyMockResponse(c, "GET", mockURL, nil)
}

func FetchShopeeOrderDetail(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/order/get_order_detail?order_sn_list=%s",
		getShopeeMockBase(c), c.Query("order_sn_list"))
	proxyMockResponse(c, "GET", mockURL, nil)
}

func ShipShopeeOrder(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/order/ship_order", getShopeeMockBase(c))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyMockResponse(c, "POST", mockURL, body)
}

func FetchShopeeShipmentList(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/order/get_shipment_list?page_size=%s&cursor=%s",
		getShopeeMockBase(c), c.DefaultQuery("page_size", "10"), c.Query("cursor"))
	proxyMockResponse(c, "GET", mockURL, nil)
}

func FetchShopeeShippingParameter(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/order/get_shipping_parameter?order_sn=%s",
		getShopeeMockBase(c), c.Query("order_sn"))
	proxyMockResponse(c, "GET", mockURL, nil)
}

func FetchShopeeTrackingNumber(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/order/get_tracking_number?order_sn=%s",
		getShopeeMockBase(c), c.Query("order_sn"))
	proxyMockResponse(c, "GET", mockURL, nil)
}

func CancelShopeeOrder(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/order/cancel_order", getShopeeMockBase(c))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyMockResponse(c, "POST", mockURL, body)
}

// --- SHOPEE PRODUCT APIS ---

func FetchShopeeItemList(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/product/get_item_list?offset=%s&page_size=%s&item_status=%s",
		getShopeeMockBase(c), c.DefaultQuery("offset", "0"), c.DefaultQuery("page_size", "10"), c.DefaultQuery("item_status", "NORMAL"))
	proxyMockResponse(c, "GET", mockURL, nil)
}

func FetchShopeeItemBaseInfo(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/product/get_item_base_info?item_id_list=%s",
		getShopeeMockBase(c), c.Query("item_id_list"))
	proxyMockResponse(c, "GET", mockURL, nil)
}

func UpdateShopeeStock(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/product/update_stock", getShopeeMockBase(c))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyMockResponse(c, "POST", mockURL, body)
}

func UpdateShopeePrice(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v2/shopee/product/update_price", getShopeeMockBase(c))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyMockResponse(c, "POST", mockURL, body)
}

func SyncProductsFromChannel(c *gin.Context) {
	var req struct {
		StoreID     uint64 `json:"store_id" binding:"required"`
		ChannelType string `json:"channel_type" binding:"required"` // SHOPEE | TIKTOK
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}

	// Gọi thẳng function Service và truyền gin.Context vào
	err := services.SyncProductsFromChannel(c, req.StoreID, req.ChannelType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi đồng bộ sản phẩm: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Đồng bộ danh sách sản phẩm từ sàn thành công!",
	})
}

// GET /api/v1/channel/unmapped-products
func GetUnmappedProducts(c *gin.Context) {
	list, err := services.GetUnmappedProducts(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi lấy danh sách chờ ghép: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": list,
	})
}

// POST /api/v1/channel/manual-map
func ManualMapProduct(c *gin.Context) {
	var req struct {
		MappingID uint64 `json:"mapping_id" binding:"required"`
		VariantID uint64 `json:"variant_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	err := services.ManualMapProduct(c, req.MappingID, req.VariantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Ghép nối thủ công thành công!",
	})
}

func ApproveAndCreateProduct(c *gin.Context) {
	var req struct {
		MappingID uint64 `json:"mapping_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}

	err := services.ApproveAndCreateProduct(c, req.MappingID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Đã phê duyệt và tạo mới sản phẩm trên Web thành công!",
	})
}
