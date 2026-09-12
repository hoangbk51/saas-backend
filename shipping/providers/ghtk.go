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
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type GHTKProvider struct {
	BaseURL    string
	httpClient *http.Client
}

// -----------------------------------------------------------------------------
// STRUCTS PAYLOAD / RESPONSE INTERNAL
// -----------------------------------------------------------------------------

type GHTKProductItem struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"` // kg
	Quantity    int     `json:"quantity"`
	ProductCode int64   `json:"product_code,omitempty"`
}

type GHTKOrderPayload struct {
	ID           string `json:"id"`
	PickName     string `json:"pick_name"`
	PickAddress  string `json:"pick_address"`
	PickTel      string `json:"pick_tel"`
	PickProvince string `json:"pick_province"`
	PickDistrict string `json:"pick_district"`
	PickWard     string `json:"pick_ward,omitempty"`
	Name         string `json:"name"`
	Tel          string `json:"tel"`
	Address      string `json:"address"`
	Province     string `json:"province"`
	District     string `json:"district"`
	Ward         string `json:"ward,omitempty"`
	Hamlet       string `json:"hamlet,omitempty"`
	IsFreeship   string `json:"is_freeship"` // "1" hoặc "0"
	PickMoney    int64  `json:"pick_money"`  // COD
	Note         string `json:"note,omitempty"`
	Value        int64  `json:"value"`                 // Giá trị khai giá
	Transport    string `json:"transport,omitempty"`   // "fly" / "road"
	PickOption   string `json:"pick_option,omitempty"` // "cod" / "post"
}

type GHTKCreateOrderPayload struct {
	Products []GHTKProductItem `json:"products"`
	Order    GHTKOrderPayload  `json:"order"`
}

type GHTKFeeData struct {
	Name         string `json:"name"`
	Fee          int64  `json:"fee"`
	InsuranceFee int64  `json:"insurance_fee"`
	Delivery     bool   `json:"delivery"`
}

type GHTKFeeResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Fee     GHTKFeeData `json:"fee"`
}

type GHTKCreateOrderData struct {
	PartnerID            string `json:"partner_id"`
	Label                string `json:"label"` // Tracking Number / Order Code GHTK
	Area                 int    `json:"area"`
	Fee                  int64  `json:"fee"`
	InsuranceFee         int64  `json:"insurance_fee"`
	EstimatedPickTime    string `json:"estimated_pick_time"`
	EstimatedDeliverTime string `json:"estimated_deliver_time"`
	StatusID             int    `json:"status_id"`
	TrackingID           int64  `json:"tracking_id"`
	SortingCode          string `json:"sorting_code"`
}

type GHTKCreateOrderResponse struct {
	Success        bool                `json:"success"`
	Message        string              `json:"message"`
	Order          GHTKCreateOrderData `json:"order"`
	WarningMessage string              `json:"warning_message"`
}

// -----------------------------------------------------------------------------
// HELPER DB MAPPING ADDRESS NAMES (CENTRAL DB)
// GHTK dùng tên trực tiếp (ProvinceName, DistrictName, WardName)
// -----------------------------------------------------------------------------

type LocationNames struct {
	ProvinceName string
	DistrictName string
	WardName     string
}

func (g *GHTKProvider) getGHTKLocationNames(c *gin.Context, districtID *int64, wardID *int64) (*LocationNames, error) {
	dbCenter, err := utils.GetCentralDB()
	if err != nil {
		return nil, fmt.Errorf("lỗi kết nối DB Center: %w", err)
	}
	utils.LogToFile(districtID)
	utils.LogToFile(wardID)

	ctx := c.Request.Context()
	loc := &LocationNames{
		//Hamlet: "Khác",
	}

	if districtID != nil && *districtID > 0 {
		query := `
			SELECT d.name AS district_name, p.name AS province_name 
			FROM districts d
			JOIN states p ON d.state_id = p.id
			WHERE d.id = ? LIMIT 1`
		err := dbCenter.QueryRowContext(ctx, query, *districtID).Scan(&loc.DistrictName, &loc.ProvinceName)
		if err != nil && err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[GHTK] Lỗi query District/Province Name: %v", err))
		}
	}

	if wardID != nil && *wardID > 0 {
		query := "SELECT name FROM wards WHERE id = ? LIMIT 1"
		err := dbCenter.QueryRowContext(ctx, query, *wardID).Scan(&loc.WardName)
		if err != nil && err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[GHTK] Lỗi query Ward Name: %v", err))
		}
	}
	utils.LogToFile(loc)

	return loc, nil
}

func (g *GHTKProvider) getGHTKSenderLocationNames(c *gin.Context) (*LocationNames, error) {
	fromDistrictStr := utils.GetSetting(c, "pickup_district_id")
	fromWardStr := utils.GetSetting(c, "pickup_ward_id")
	//fromDistrictStr = "1"
	//fromWardStr = ""
	var appDistrictID *int64
	if id, err := strconv.ParseInt(fromDistrictStr, 10, 64); err == nil && id > 0 {
		appDistrictID = &id
	}

	var appWardID *int64
	if id, err := strconv.ParseInt(fromWardStr, 10, 64); err == nil && id > 0 {
		appWardID = &id
	}

	loc, err := g.getGHTKLocationNames(c, appDistrictID, appWardID)
	if err != nil {
		return nil, err
	}

	// Fallback từ setting nếu có sẵn tên trực tiếp trong config
	if loc.ProvinceName == "" {
		loc.ProvinceName, _ = utils.GetPluginSetting(c, "ghtk", "sender_province_name")
		loc.ProvinceName = "Hà Nội" //
	}
	if loc.DistrictName == "" {
		loc.DistrictName, _ = utils.GetPluginSetting(c, "ghtk", "sender_district_name")
	}
	if loc.WardName == "" {
		loc.WardName, _ = utils.GetPluginSetting(c, "ghtk", "sender_ward_name")
	}

	return loc, nil
}

// -----------------------------------------------------------------------------
// HELPER SETTINGS & REQUEST
// -----------------------------------------------------------------------------

func (g *GHTKProvider) getCredentials(c *gin.Context) (token string, baseURL string) {
	isLiveStr, _ := utils.GetPluginSetting(c, "ghtk", "ghtk_is_live")
	isLive, _ := strconv.Atoi(isLiveStr)

	if isLive == 1 {
		token, _ = utils.GetPluginSetting(c, "ghtk", "ghtk_live_token")
		baseURL = "https://services.giaohangtietkiem.vn"
	} else {
		token, _ = utils.GetPluginSetting(c, "ghtk", "ghtk_test_token")
		// Sandbox GHTK URL hoặc cấu hình trực tiếp live nếu sandbox chập chờn
		baseURL = "https://services-staging.ghtklab.com"
		if token == "" {
			// Fallback token nếu chưa lưu setting
			//token, _ = utils.GetPluginSetting(c, "ghtk", "ghtk_live_token")
			//baseURL = "https://services.giaohangtietkiem.vn"
		}
	}

	return token, baseURL
}

func (g *GHTKProvider) doRequest(c *gin.Context, method, path string, queryParams url.Values, payload interface{}) ([]byte, error) {
	utils.LogToFile("doRequest GHTK")

	token, baseURL := g.getCredentials(c)
	utils.LogToFile("finisih getCredentials")

	utils.LogToFile(queryParams)
	utils.LogToFile("payload")

	var bodyReader io.Reader
	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload error: %w", err)
		}
		utils.LogToFile(fmt.Sprintf("[GHTK Request Payload] %s %s: %s", method, path, string(jsonBytes)))
		bodyReader = bytes.NewBuffer(jsonBytes)
	}

	fullURL := baseURL + path
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create http request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", token)

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

	// Log response dạng string readable
	utils.LogToFile(fmt.Sprintf("[GHTK Response Body] %s %s: %s", method, path, string(body)))

	return body, nil
}

// -----------------------------------------------------------------------------
// PUBLIC METHODS (IMPLEMENT INTERFACE)
// -----------------------------------------------------------------------------

func (g *GHTKProvider) GetName() string {
	return "Giao Hàng Tiết Kiệm"
}

func (g *GHTKProvider) CalculateFee(c *gin.Context, cart *models.Cart) (float64, error) {
	senderLoc, err := g.getGHTKSenderLocationNames(c)
	if err != nil {
		return 0, fmt.Errorf("map sender location error: %w", err)
	}

	receiverLoc, err := g.getGHTKLocationNames(c, cart.ShippingDistrictID, cart.ShippingWardID)
	if err != nil {
		return 0, fmt.Errorf("map receiver location error: %w", err)
	}

	totalWeightGram := cart.ShippingWeight
	if totalWeightGram <= 0 {
		totalWeightGram = 200 // Mặc định 200g
	}

	params := url.Values{}
	params.Set("pick_province", senderLoc.ProvinceName)
	params.Set("pick_district", senderLoc.DistrictName)
	params.Set("province", receiverLoc.ProvinceName)
	params.Set("district", receiverLoc.DistrictName)
	params.Set("address", cart.ShippingAddress)
	params.Set("weight", strconv.FormatInt(int64(totalWeightGram), 10))
	params.Set("value", strconv.FormatInt(int64(cart.GrandTotal), 10))
	params.Set("transport", "road") // Mặc định road (đường bộ) hoặc fly (đường bay)

	if receiverLoc.WardName != "" {
		params.Set("ward", receiverLoc.WardName)
	}

	endpoint := "/services/shipment/fee"
	utils.LogToFile(params)

	respBody, err := g.doRequest(c, http.MethodGet, endpoint, params, nil)
	if err != nil {
		return 0, err
	}

	var result GHTKFeeResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("unmarshal ghtk fee res error: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("ghtk fee error: %s", result.Message)
	}

	return float64(result.Fee.Fee), nil
}

func (g *GHTKProvider) GetServices(c *gin.Context, cart *models.Cart) ([]models.ShippingService, error) {
	fee, err := g.CalculateFee(c, cart)
	if err != nil {
		return nil, err
	}

	return []models.ShippingService{
		{
			Code:  "ghtk_standard",
			Name:  "Giao Hàng Tiết Kiệm (Tiêu chuẩn)",
			Value: fee,
		},
	}, nil
}

func (g *GHTKProvider) CreateOrder(c *gin.Context, order *models.Order, note string) (string, float64, error) {
	senderLoc, err := g.getGHTKSenderLocationNames(c)
	if err != nil {
		return "", 0, fmt.Errorf("map sender location error: %w", err)
	}

	receiverLoc, err := g.getGHTKLocationNames(c, order.ShippingCity, order.ShippingWard)
	if err != nil {
		return "", 0, fmt.Errorf("map receiver location error: %w", err)
	}

	senderName, _ := utils.GetPluginSetting(c, "ghtk", "sender_name")
	senderPhone, _ := utils.GetPluginSetting(c, "ghtk", "sender_phone")
	senderAddress, _ := utils.GetPluginSetting(c, "ghtk", "sender_address")
	senderName = "Hoang"
	senderPhone = "0382305145"
	senderAddress = "90B Đội Cấn"
	totalWeightGram := order.TotalWeight
	if totalWeightGram <= 0 {
		totalWeightGram = 200
	}
	weightKg := float64(totalWeightGram) / 1000.0

	codAmount := int64(0)
	if order.PaymentMethodCode == "cod" {
		codAmount = int64(order.GrandTotal)
	}

	payload := &GHTKCreateOrderPayload{
		Products: []GHTKProductItem{
			{
				Name:     fmt.Sprintf("Đơn hàng %s", order.OrderNumber),
				Weight:   weightKg,
				Quantity: 1,
			},
		},
		Order: GHTKOrderPayload{
			Hamlet:       "Khác",
			ID:           order.OrderNumber,
			PickName:     senderName,
			PickTel:      senderPhone,
			PickAddress:  senderAddress,
			PickProvince: senderLoc.ProvinceName,
			PickDistrict: senderLoc.DistrictName,
			PickWard:     senderLoc.WardName,
			Name:         fmt.Sprintf("%s %s", order.ShippingFirstName, order.ShippingLastName),
			Tel:          order.CustomerPhone,
			Address:      order.ShippingAddress1,
			Province:     receiverLoc.ProvinceName,
			District:     receiverLoc.DistrictName,
			Ward:         receiverLoc.WardName,
			IsFreeship:   "0",
			PickMoney:    codAmount,
			Note:         note,
			Value:        int64(order.GrandTotal),
			Transport:    "road",
			PickOption:   "cod",
		},
	}

	endpoint := "/services/shipment/order"
	respBody, err := g.doRequest(c, http.MethodPost, endpoint, nil, payload)
	if err != nil {
		return "", 0, err
	}

	var result GHTKCreateOrderResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, fmt.Errorf("unmarshal ghtk create order res error: %w", err)
	}

	if !result.Success {
		return "", 0, fmt.Errorf("ghtk create order failed: %s", result.Message)
	}

	// `result.Order.Label` chính là mã vận đơn GHTK (ví dụ: S360061.SGP39-Q29.1761788348)
	trackingCode := result.Order.Label
	if trackingCode == "" {
		trackingCode = strconv.FormatInt(result.Order.TrackingID, 10)
	}

	return trackingCode, float64(result.Order.Fee), nil
}

func (g *GHTKProvider) CancelShipment(c *gin.Context, trackingCode string) error {
	return nil
}

// -----------------------------------------------------------------------------
// AUTO REGISTER IN INIT
// -----------------------------------------------------------------------------

func init() {
	utils.LogToFile("init GHTKProvider")
	provider := &GHTKProvider{
		BaseURL: "https://services.giaohangtietkiem.vn",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	shipping.Register("ghtk", provider)
}
