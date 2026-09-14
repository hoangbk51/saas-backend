package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

// Helper lấy base URL TikTok Mock
func getTikTokMockBase(c *gin.Context) string {
	return utils.GetSetting(c, "tiktok_mock_url", "http://localhost:3003/api/v1")
}

// Helper gọi mock TikTok và trả JSON response trực tiếp ra Controller
func proxyTikTokMockResponse(c *gin.Context, method string, mockURL string, body interface{}) {
	headers := map[string]string{
		"x-tts-access-token": c.GetHeader("Authorization"),
	}

	respBytes, err := utils.RequestMock(c, method, mockURL, headers, body)
	if err != nil && len(respBytes) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi parse dữ liệu từ Mock TikTok Server"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// -----------------------------------------------------------------------------
// 1. CONNECT URL API
// -----------------------------------------------------------------------------

func GetTikTokConnectURL(c *gin.Context) {
	appKey := utils.GetSetting(c, "tiktok_app_key", "mock_app_key_123")
	storeName := utils.GetSetting(c, "store_name", "Cửa hàng")
	authURL := fmt.Sprintf("%s/oauth/authorize?app_key=%s&state=%s", getTikTokMockBase(c), appKey, storeName)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "auth_url": authURL})
}

// -----------------------------------------------------------------------------
// 2. TIKTOK ORDER APIS
// -----------------------------------------------------------------------------

func SearchTikTokOrders(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v1/tiktok/order/search?page_size=%s",
		getTikTokMockBase(c), c.DefaultQuery("page_size", "10"))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyTikTokMockResponse(c, "POST", mockURL, body)
}

func FetchTikTokOrderDetail(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v1/tiktok/order/detail?order_ids=%s",
		getTikTokMockBase(c), c.Query("order_ids"))
	proxyTikTokMockResponse(c, "GET", mockURL, nil)
}

func ShipTikTokOrder(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v1/tiktok/order/ship", getTikTokMockBase(c))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyTikTokMockResponse(c, "POST", mockURL, body)
}

// -----------------------------------------------------------------------------
// 3. TIKTOK PRODUCT APIS
// -----------------------------------------------------------------------------

func SearchTikTokProducts(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v1/tiktok/product/search?page_size=%s",
		getTikTokMockBase(c), c.DefaultQuery("page_size", "10"))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyTikTokMockResponse(c, "POST", mockURL, body)
}

func UpdateTikTokStock(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v1/tiktok/product/stock/update", getTikTokMockBase(c))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyTikTokMockResponse(c, "POST", mockURL, body)
}

func UpdateTikTokPrice(c *gin.Context) {
	mockURL := fmt.Sprintf("%s/api/v1/tiktok/product/price/update", getTikTokMockBase(c))
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	proxyTikTokMockResponse(c, "POST", mockURL, body)
}
