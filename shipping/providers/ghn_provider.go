package providers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/shipping"
	"go-saas/utils"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type GHNProvider struct {
	BaseURL    string
	httpClient *http.Client
}

// -----------------------------------------------------------------------------
// STRUCTS PAYLOAD / RESPONSE INTERNAL
// -----------------------------------------------------------------------------

type GHNItem struct {
	Name     string `json:"name"`
	Code     string `json:"code,omitempty"`
	Quantity int    `json:"quantity"`
	Price    int64  `json:"price,omitempty"`
	Weight   int    `json:"weight"`
	Length   int    `json:"length,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

type GHNAvailableServicesPayload struct {
	ShopID       int64 `json:"shop_id"`
	FromDistrict int64 `json:"from_district"`
	ToDistrict   int64 `json:"to_district"`
}

type GHNServiceItem struct {
	ServiceID     int64  `json:"service_id"`
	ShortName     string `json:"short_name"`
	ServiceTypeID int    `json:"service_type_id"`
}

type GHNCalculateFeePayload struct {
	FromDistrictID int64     `json:"from_district_id"`
	FromWardCode   string    `json:"from_ward_code"`
	ToDistrictID   int64     `json:"to_district_id"`
	ToWardCode     string    `json:"to_ward_code"`
	ServiceID      int64     `json:"service_id,omitempty"`
	ServiceTypeID  int       `json:"service_type_id,omitempty"`
	Weight         int       `json:"weight"`
	Length         int       `json:"length"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	InsuranceValue int64     `json:"insurance_value"`
	Items          []GHNItem `json:"items,omitempty"`
}

type GHNCreateOrderPayload struct {
	PaymentTypeID   int       `json:"payment_type_id"`
	Note            string    `json:"note"`
	RequiredNote    string    `json:"required_note"`
	ClientOrderCode string    `json:"client_order_code"`
	ToName          string    `json:"to_name"`
	ToPhone         string    `json:"to_phone"`
	ToAddress       string    `json:"to_address"`
	ToWardCode      string    `json:"to_ward_code"`
	ToDistrictID    int64     `json:"to_district_id"`
	CodAmount       int64     `json:"cod_amount"`
	Content         string    `json:"content"`
	Weight          int       `json:"weight"`
	Length          int       `json:"length"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	InsuranceValue  int64     `json:"insurance_value"`
	ServiceID       int64     `json:"service_id,omitempty"`
	ServiceTypeID   int       `json:"service_type_id,omitempty"`
	Items           []GHNItem `json:"items"`
}

type GHNResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type GHNFeeData struct {
	Total      int64 `json:"total"`
	ServiceFee int64 `json:"service_fee"`
	Insurance  int64 `json:"insurance_fee"`
}

type GHNCreateOrderData struct {
	OrderCode            string `json:"order_code"`
	TotalFee             int64  `json:"total_fee"`
	ExpectedDeliveryTime string `json:"expected_delivery_time"`
}

// -----------------------------------------------------------------------------
// HELPER DB MAPPING LOCATIONS (CENTRAL DB)
// -----------------------------------------------------------------------------

func (g *GHNProvider) getGHNMappingIDs(c *gin.Context, shippingDistrictID *int64, shippingWardID *int64) (int64, string, error) {
	dbCenter, err := utils.GetCentralDB()
	if err != nil {
		return 0, "", fmt.Errorf("lỗi kết nối DB Center: %w", err)
	}

	ctx := c.Request.Context()
	var toDistrictID int64
	var toWardCode string

	if shippingDistrictID != nil {
		query := "SELECT DistrictID FROM ghn_districts WHERE app_district_id = ? LIMIT 1"
		err := dbCenter.QueryRowContext(ctx, query, *shippingDistrictID).Scan(&toDistrictID)
		if err != nil && err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[GHN] Lỗi query DistrictID: %v", err))
		}
	}

	if shippingWardID != nil {
		query := "SELECT WardCode FROM ghn_wards WHERE ward_app_id = ? LIMIT 1"
		err := dbCenter.QueryRowContext(ctx, query, *shippingWardID).Scan(&toWardCode)
		if err != nil && err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[GHN] Lỗi query WardCode: %v", err))
		}
	}

	return toDistrictID, toWardCode, nil
}

func (g *GHNProvider) getGHNSenderMappingIDs(c *gin.Context) (int64, string, error) {
	fromDistrictStr, _ := utils.GetPluginSetting(c, "ghn", "sender_district_id")
	fromWardStr, _ := utils.GetPluginSetting(c, "ghn", "sender_ward_code")
	fromDistrictStr = "18"
	fromWardStr = "9874"
	var appDistrictID *int64
	if id, err := strconv.ParseInt(fromDistrictStr, 10, 64); err == nil && id > 0 {
		appDistrictID = &id
	}

	var appWardID *int64
	if id, err := strconv.ParseInt(fromWardStr, 10, 64); err == nil && id > 0 {
		appWardID = &id
	}

	fromDistrictID, fromWardCode, err := g.getGHNMappingIDs(c, appDistrictID, appWardID)
	if err != nil {
		return 0, "", err
	}

	// Fallback nếu setting lưu trực tiếp GHN WardCode (dạng string) thay vì ID
	if fromWardCode == "" && fromWardStr != "" {
		fromWardCode = fromWardStr
	}

	return fromDistrictID, fromWardCode, nil
}

// -----------------------------------------------------------------------------
// HELPER SETTINGS & REQUEST
// -----------------------------------------------------------------------------

func (g *GHNProvider) getCredentials(c *gin.Context) (token string, baseURL string, shopID int64) {
	isLiveStr, _ := utils.GetPluginSetting(c, "ghn", "giaohangnhanh_is_live")
	isLive, _ := strconv.Atoi(isLiveStr)

	if isLive == 1 {
		token, _ = utils.GetPluginSetting(c, "ghn", "giaohangnhanh_live_token")
		baseURL = "https://online-gateway.ghn.vn/shiip/public-api"
		shopIDStr, _ := utils.GetPluginSetting(c, "ghn", "live_shop_id")
		shopID, _ = strconv.ParseInt(shopIDStr, 10, 64)
	} else {
		token, _ = utils.GetPluginSetting(c, "ghn", "giaohangnhanh_test_token")
		baseURL = "https://dev-online-gateway.ghn.vn/shiip/public-api"
		shopIDStr, _ := utils.GetPluginSetting(c, "ghn", "test_shop_id")
		if shopIDStr == "" {
			shopIDStr = "199032"
		}
		shopID, _ = strconv.ParseInt(shopIDStr, 10, 64)
	}

	return token, baseURL, shopID
}

func (g *GHNProvider) doRequest(c *gin.Context, method, path string, payload interface{}) ([]byte, error) {
	token, baseURL, shopID := g.getCredentials(c)
	utils.LogToFile(payload)
	var bodyReader io.Reader
	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload error: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBytes)
	}

	url := baseURL + path
	req, err := http.NewRequestWithContext(c.Request.Context(), method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create http request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", token)
	req.Header.Set("ShopId", strconv.FormatInt(shopID, 10))

	httpClient := g.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body error: %w", err)
	}

	return body, nil
}

// -----------------------------------------------------------------------------
// GHN API METHODS
// -----------------------------------------------------------------------------

func (g *GHNProvider) GetAvailableServices(c *gin.Context, fromDistrictID, toDistrictID int64) ([]GHNServiceItem, error) {
	utils.LogToFile("GetAvailableServices GHN")
	_, _, shopID := g.getCredentials(c)

	payload := &GHNAvailableServicesPayload{
		ShopID:       shopID,
		FromDistrict: fromDistrictID,
		ToDistrict:   toDistrictID,
	}
	utils.LogToFile(payload)
	endpoint := "/v2/shipping-order/available-services"
	respBody, err := g.doRequest(c, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, err
	}
	utils.LogToFile(fmt.Sprintf("[GHN Response] %s", string(respBody)))
	var result GHNResponse[[]GHNServiceItem]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal available services res error: %w", err)
	}

	if result.Code != 200 {
		return nil, fmt.Errorf("ghn get services error [%d]: %s", result.Code, result.Message)
	}

	utils.LogToFile(result)
	return result.Data, nil
}

func (g *GHNProvider) CalculateFeeForService(c *gin.Context, cart *models.Cart, serviceID int64, serviceTypeID int) (float64, error) {
	fromDistrictID, fromWardCode, err := g.getGHNSenderMappingIDs(c)
	if err != nil {
		return 0, fmt.Errorf("map sender location error: %w", err)
	}

	toDistrictID, toWardCode, err := g.getGHNMappingIDs(c, cart.ShippingDistrictID, cart.ShippingWardID)
	if err != nil {
		return 0, fmt.Errorf("map receiver location error: %w", err)
	}

	totalWeight := int(cart.ShippingWeight)
	if totalWeight <= 0 {
		totalWeight = 200
	}

	payload := &GHNCalculateFeePayload{
		FromDistrictID: fromDistrictID,
		FromWardCode:   fromWardCode,
		ToDistrictID:   toDistrictID,
		ToWardCode:     toWardCode,
		ServiceID:      serviceID,
		ServiceTypeID:  serviceTypeID,
		Weight:         totalWeight,
		Length:         2,
		Width:          2,
		Height:         2,
		InsuranceValue: int64(cart.GrandTotal),
	}
	utils.LogToFile(payload)
	endpoint := "/v2/shipping-order/fee"
	respBody, err := g.doRequest(c, http.MethodPost, endpoint, payload)
	if err != nil {
		return 0, err
	}

	var result GHNResponse[GHNFeeData]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("unmarshal ghn fee res error: %w", err)
	}

	if result.Code != 200 {
		return 0, fmt.Errorf("ghn error [%d]: %s", result.Code, result.Message)
	}
	utils.LogToFile("result GHN")

	utils.LogToFile(result)

	return float64(result.Data.Total), nil
}

// -----------------------------------------------------------------------------
// PUBLIC METHODS (IMPLEMENT INTERFACE)
// -----------------------------------------------------------------------------

func (g *GHNProvider) GetName() string {
	return "Giao Hàng Nhanh"
}

func (g *GHNProvider) GetServices(c *gin.Context, cart *models.Cart) ([]models.ShippingService, error) {
	fromDistrictID, _, err := g.getGHNSenderMappingIDs(c)
	if err != nil {
		return nil, err
	}

	toDistrictID, _, err := g.getGHNMappingIDs(c, cart.ShippingDistrictID, cart.ShippingWardID)
	if err != nil {
		return nil, err
	}

	ghnServices, err := g.GetAvailableServices(c, fromDistrictID, toDistrictID)
	if err != nil || len(ghnServices) == 0 {
		fee, err := g.CalculateFee(c, cart)
		if err != nil {
			return nil, err
		}
		return []models.ShippingService{
			{
				Code:  "express",
				Name:  "Giao Hàng Nhanh",
				Value: fee,
			},
		}, nil
	}

	var services []models.ShippingService
	for _, s := range ghnServices {
		utils.LogToFile(s.ShortName)

		fee, err := g.CalculateFeeForService(c, cart, s.ServiceID, s.ServiceTypeID)

		if err != nil {
			utils.LogToFile(err)

			continue
		}
		services = append(services, models.ShippingService{
			Code:  fmt.Sprintf("ghn_%d", s.ServiceID),
			Name:  fmt.Sprintf("GHN - %s", s.ShortName),
			Value: fee,
		})
	}
	utils.LogToFile(services)
	return services, nil
}

func (g *GHNProvider) CalculateFee(c *gin.Context, cart *models.Cart) (float64, error) {
	fromDistrictID, fromWardCode, err := g.getGHNSenderMappingIDs(c)
	if err != nil {
		return 0, fmt.Errorf("map sender location error: %w", err)
	}

	toDistrictID, toWardCode, err := g.getGHNMappingIDs(c, cart.ShippingDistrictID, cart.ShippingWardID)
	if err != nil {
		return 0, fmt.Errorf("map receiver location error: %w", err)
	}

	var serviceID int64
	var serviceTypeID int = 2

	availServices, err := g.GetAvailableServices(c, fromDistrictID, toDistrictID)
	if err == nil && len(availServices) > 0 {
		serviceID = availServices[0].ServiceID
		serviceTypeID = availServices[0].ServiceTypeID
	}

	totalWeight := int(cart.ShippingWeight)
	if totalWeight <= 0 {
		totalWeight = 200
	}

	payload := &GHNCalculateFeePayload{
		FromDistrictID: fromDistrictID,
		FromWardCode:   fromWardCode,
		ToDistrictID:   toDistrictID,
		ToWardCode:     toWardCode,
		ServiceID:      serviceID,
		ServiceTypeID:  serviceTypeID,
		Weight:         totalWeight,
		Length:         10,
		Width:          10,
		Height:         10,
		InsuranceValue: int64(cart.GrandTotal),
	}

	endpoint := "/v2/shipping-order/fee"
	respBody, err := g.doRequest(c, http.MethodPost, endpoint, payload)
	if err != nil {
		return 0, err
	}

	var result GHNResponse[GHNFeeData]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("unmarshal ghn fee res error: %w", err)
	}

	if result.Code != 200 {
		return 0, fmt.Errorf("ghn error [%d]: %s", result.Code, result.Message)
	}

	return float64(result.Data.Total), nil
}

func (g *GHNProvider) CreateOrder(c *gin.Context, order *models.Order, note string) (string, float64, error) {
	requiredNote, _ := utils.GetPluginSetting(c, "ghn", "required_note")
	if requiredNote == "" {
		requiredNote = "KHONGCHOXEMHANG"
	}

	toDistrictID, toWardCode, err := g.getGHNMappingIDs(c, order.ShippingCity, order.ShippingWard)
	if err != nil {
		return "", 0, fmt.Errorf("map receiver order location error: %w", err)
	}

	totalWeight := int(order.TotalWeight)
	if totalWeight <= 0 {
		totalWeight = 200
	}

	paymentTypeID := 1
	codAmount := int64(0)
	if order.PaymentMethodCode == "cod" {
		codAmount = int64(order.GrandTotal)
	}
	utils.LogToFile(order.OrderNumber)
	payload := &GHNCreateOrderPayload{
		PaymentTypeID:   paymentTypeID,
		Note:            note,
		RequiredNote:    requiredNote,
		ClientOrderCode: order.OrderNumber,
		ToName:          fmt.Sprintf("%s %s", order.ShippingFirstName, order.ShippingLastName),
		ToPhone:         order.CustomerPhone,
		ToAddress:       order.ShippingAddress1,
		ToWardCode:      toWardCode,
		ToDistrictID:    toDistrictID,
		CodAmount:       codAmount,
		Content:         fmt.Sprintf("Don hang %s", order.OrderNumber),
		Weight:          totalWeight,
		Length:          10,
		Width:           10,
		Height:          10,
		InsuranceValue:  int64(order.GrandTotal),
		ServiceTypeID:   2,
	}

	endpoint := "/v2/shipping-order/create"
	respBody, err := g.doRequest(c, http.MethodPost, endpoint, payload)
	if err != nil {
		return "", 0, err
	}

	var result GHNResponse[GHNCreateOrderData]
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, fmt.Errorf("unmarshal ghn create order res error: %w", err)
	}

	if result.Code != 200 {
		return "", 0, fmt.Errorf("ghn create order failed [%d]: %s", result.Code, result.Message)
	}

	return result.Data.OrderCode, float64(result.Data.TotalFee), nil
}

// -----------------------------------------------------------------------------
// AUTO REGISTER IN INIT
// -----------------------------------------------------------------------------

func init() {
	utils.LogToFile("init GHNProvider")
	provider := &GHNProvider{
		BaseURL: "https://online-gateway.ghn.vn/shiip/public-api",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	shipping.Register("ghn", provider)
}

func (g *GHNProvider) CancelShipment(c *gin.Context, trackingCode string) error {
	return nil
}
