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

type GoShipProvider struct {
	httpClient *http.Client
}

// -----------------------------------------------------------------------------
// STRUCTS PAYLOAD / RESPONSE INTERNAL (GOSHIP API V2)
// -----------------------------------------------------------------------------

type GoShipAddress struct {
	Name     string `json:"name,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Street   string `json:"street,omitempty"`
	City     string `json:"city"`           // GoShip State/City ID (String)
	District string `json:"district"`       // GoShip District ID (String)
	Ward     string `json:"ward,omitempty"` // GoShip Ward ID (String)
}

type GoShipParcel struct {
	Weight   int         `json:"weight"` // gram
	Width    int         `json:"width"`  // cm
	Height   int         `json:"height"` // cm
	Length   int         `json:"length"` // cm
	Cod      int64       `json:"cod"`    // VNĐ
	Amount   int64       `json:"amount"` // VNĐ (Giá trị khai giá)
	Metadata interface{} `json:"metadata,omitempty"`
}

type GoShipRateShipment struct {
	AddressFrom GoShipAddress `json:"address_from"`
	AddressTo   GoShipAddress `json:"address_to"`
	Parcel      GoShipParcel  `json:"parcel"`
}

type GoShipRatePayload struct {
	Shipment GoShipRateShipment `json:"shipment"`
}

type GoShipRateItem struct {
	ID          string  `json:"id"` // Rate ID (dùng để book đơn)
	CarrierName string  `json:"carrier_name"`
	CarrierLogo string  `json:"carrier_logo"`
	Service     string  `json:"service"`
	TotalAmount float64 `json:"total_amount"`
	TotalFee    float64 `json:"total_fee"`
	Discount    float64 `json:"discount"`
	Expected    string  `json:"expected"`
}

type GoShipRateResponse struct {
	Code    int              `json:"code"`
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Data    []GoShipRateItem `json:"data"`
}

type GoShipCreateOrderShipment struct {
	Rate        string        `json:"rate,omitempty"` // Mã Rate ID từ API /rates
	Payer       int           `json:"payer"`          // 0: Shop trả phí, 1: Người nhận trả phí
	AddressFrom GoShipAddress `json:"address_from"`
	AddressTo   GoShipAddress `json:"address_to"`
	Parcel      GoShipParcel  `json:"parcel"`
}

type GoShipCreateOrderPayload struct {
	Shipment GoShipCreateOrderShipment `json:"shipment"`
}

type GoShipCreateOrderResponse struct {
	Code             int     `json:"code"`
	Status           string  `json:"status"`
	Message          string  `json:"message"`
	ID               string  `json:"id"` // GoShip Order Reference Code (VD: GS8E23EG18)
	TrackingNumber   string  `json:"tracking_number"`
	Carrier          string  `json:"carrier"`
	CarrierShortName string  `json:"carrier_short_name"`
	Fee              float64 `json:"fee"`
}

// -----------------------------------------------------------------------------
// HELPER DB MAPPING (LOCATION MAPPER)
// -----------------------------------------------------------------------------

type GoShipLocation struct {
	StateID    string
	DistrictID string
	WardID     string
}

func (g *GoShipProvider) getGoShipSenderLocation(c *gin.Context) (*GoShipLocation, error) {
	fromStateStr := utils.GetSetting(c, "pickup_state_id")
	fromDistrictStr := utils.GetSetting(c, "pickup_district_id")
	fromWardStr := utils.GetSetting(c, "pickup_ward_id")

	var appStateID, appDistrictID, appWardID *int64

	if id, err := strconv.ParseInt(fromStateStr, 10, 64); err == nil && id > 0 {
		appStateID = &id
	}
	if id, err := strconv.ParseInt(fromDistrictStr, 10, 64); err == nil && id > 0 {
		appDistrictID = &id
	}
	if id, err := strconv.ParseInt(fromWardStr, 10, 64); err == nil && id > 0 {
		appWardID = &id
	}

	return g.getGoShipLocation(c, appDistrictID, appStateID, appWardID)
}

func (g *GoShipProvider) getGoShipLocation(c *gin.Context, appDistrictID *int64, appStateID *int64, appWardID *int64) (*GoShipLocation, error) {
	dbCenter, err := utils.GetCentralDB()
	if err != nil {
		return nil, fmt.Errorf("lỗi kết nối DB Center: %w", err)
	}

	ctx := c.Request.Context()
	loc := &GoShipLocation{}

	// 1. Query GoShip State ID từ app_state_id
	if appStateID != nil && *appStateID > 0 {
		var stateID int64
		queryState := "SELECT id FROM goship_states WHERE app_state_id = ? LIMIT 1"
		if err := dbCenter.QueryRowContext(ctx, queryState, *appStateID).Scan(&stateID); err == nil {
			loc.StateID = strconv.FormatInt(stateID, 10)
		} else if err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[GoShip] Lỗi query goship_states: %v", err))
		}
	}

	// 2. Query GoShip District ID (và fallback lấy StateID nếu chưa có)
	if appDistrictID != nil && *appDistrictID > 0 {
		var districtID, stateID int64
		queryDistrict := "SELECT id, state_id FROM goship_districts WHERE app_district_id = ? LIMIT 1"
		if err := dbCenter.QueryRowContext(ctx, queryDistrict, *appDistrictID).Scan(&districtID, &stateID); err == nil {
			loc.DistrictID = strconv.FormatInt(districtID, 10)
			if loc.StateID == "" && stateID > 0 {
				loc.StateID = strconv.FormatInt(stateID, 10)
			}
		} else if err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[GoShip] Lỗi query goship_districts: %v", err))
		}
	}

	// 3. Query GoShip Ward ID
	if appWardID != nil && *appWardID > 0 {
		var wardID int64
		queryWard := "SELECT id FROM goship_wards WHERE app_ward_id = ? LIMIT 1"
		if err := dbCenter.QueryRowContext(ctx, queryWard, *appWardID).Scan(&wardID); err == nil {
			loc.WardID = strconv.FormatInt(wardID, 10)
		} else if err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[GoShip] Lỗi query goship_wards: %v", err))
		}
	}
	utils.LogToFile(loc)
	return loc, nil
}

// -----------------------------------------------------------------------------
// HELPER SETTINGS & REQUEST
// -----------------------------------------------------------------------------

func (g *GoShipProvider) getCredentials(c *gin.Context) (token string, baseURL string) {
	isLiveStr, _ := utils.GetPluginSetting(c, "goship", "goship_is_live")
	isLive, _ := strconv.Atoi(isLiveStr)

	if isLive == 1 {
		token, _ = utils.GetPluginSetting(c, "goship", "goship_live_token")
		baseURL = "https://api.goship.io/api/v2"
	} else {
		token, _ = utils.GetPluginSetting(c, "goship", "goship_test_token")
		baseURL = "https://sandbox.goship.io/api/v2"
	}

	return token, baseURL
}

func (g *GoShipProvider) doRequest(c *gin.Context, method, path string, payload interface{}) ([]byte, error) {
	token, baseURL := g.getCredentials(c)

	var bodyReader io.Reader
	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload error: %w", err)
		}
		utils.LogToFile(fmt.Sprintf("[GoShip Request Payload] %s %s: %s", method, path, string(jsonBytes)))
		bodyReader = bytes.NewBuffer(jsonBytes)
	}

	fullURL := baseURL + path
	req, err := http.NewRequestWithContext(c.Request.Context(), method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create http request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

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

	utils.LogToFile(fmt.Sprintf("[GoShip Response Body] %s %s: %s", method, path, string(body)))

	return body, nil
}

// -----------------------------------------------------------------------------
// PUBLIC METHODS (IMPLEMENT INTERFACE)
// -----------------------------------------------------------------------------

func (g *GoShipProvider) GetName() string {
	return "GoShip"
}

func (g *GoShipProvider) GetServices(c *gin.Context, cart *models.Cart) ([]models.ShippingService, error) {
	senderLoc, err := g.getGoShipSenderLocation(c)
	utils.LogToFile("senderLoc")

	utils.LogToFile(senderLoc)

	if err != nil {
		return nil, fmt.Errorf("map sender location error: %w", err)
	}

	receiverLoc, err := g.getGoShipLocation(c, cart.ShippingDistrictID, cart.ShippingStateID, cart.ShippingWardID)
	if err != nil {
		return nil, fmt.Errorf("map receiver location error: %w", err)
	}

	weightInt := int(cart.ShippingWeight)
	if weightInt <= 0 {
		weightInt = 1000 // Mặc định 1000g
	}

	payload := GoShipRatePayload{
		Shipment: GoShipRateShipment{
			AddressFrom: GoShipAddress{
				City:     senderLoc.StateID,
				District: senderLoc.DistrictID,
			},
			AddressTo: GoShipAddress{
				City:     receiverLoc.StateID,
				District: receiverLoc.DistrictID,
			},
			Parcel: GoShipParcel{
				Weight: weightInt,
				Width:  10,
				Height: 10,
				Length: 10,
				Cod:    int64(cart.GrandTotal),
				Amount: int64(cart.GrandTotal),
			},
		},
	}

	respBody, err := g.doRequest(c, http.MethodPost, "/rates", payload)
	if err != nil {
		return nil, err
	}

	var result GoShipRateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal goship rates res error: %w", err)
	}

	if result.Code != 200 || len(result.Data) == 0 {
		return nil, fmt.Errorf("goship rate error: %s", result.Message)
	}

	var services []models.ShippingService
	for _, rate := range result.Data {
		services = append(services, models.ShippingService{
			Code:  rate.ID, // Mã Rate ID duy nhất từ GoShip để dùng book đơn sau này
			Name:  fmt.Sprintf("%s (%s)", rate.CarrierName, rate.Service),
			Value: rate.TotalFee,
		})
	}

	return services, nil
}

func (g *GoShipProvider) CalculateFee(c *gin.Context, cart *models.Cart) (float64, error) {
	services, err := g.GetServices(c, cart)
	if err != nil || len(services) == 0 {
		return 0, err
	}
	// Trả về phí của đơn vị vận chuyển rẻ nhất
	return services[0].Value, nil
}

func (g *GoShipProvider) CreateOrder(c *gin.Context, order *models.Order, note string) (string, float64, error) {
	senderLoc, err := g.getGoShipSenderLocation(c)
	if err != nil {
		return "", 0, fmt.Errorf("map sender location error: %w", err)
	}

	receiverLoc, err := g.getGoShipLocation(c, order.ShippingCity, order.ShippingState, order.ShippingWard)
	if err != nil {
		return "", 0, fmt.Errorf("map receiver location error: %w", err)
	}

	senderName, _ := utils.GetPluginSetting(c, "goship", "sender_name")
	senderPhone, _ := utils.GetPluginSetting(c, "goship", "sender_phone")
	senderAddress, _ := utils.GetPluginSetting(c, "goship", "sender_address")
	senderName = "hoang"
	senderPhone = "0382305124"
	senderAddress = "5-81 Thien Loi"
	weightInt := int(order.TotalWeight)

	if weightInt <= 0 {
		weightInt = 1000
	}

	codAmount := int64(0)
	if order.PaymentMethodCode == "cod" {
		codAmount = int64(order.GrandTotal)
	}

	// Mã rate ID được lưu từ bước chọn phương thức vận chuyển của khách hàng
	rateID := "MTFfMjdfMjM4Nw==" // order.ShippingMethodCode

	payload := GoShipCreateOrderPayload{
		Shipment: GoShipCreateOrderShipment{
			Rate:  rateID,
			Payer: 0, // Shop trả phí
			AddressFrom: GoShipAddress{
				Name:     senderName,
				Phone:    senderPhone,
				Street:   senderAddress,
				City:     senderLoc.StateID, // 👈 Map StateID vào City
				District: senderLoc.DistrictID,
				Ward:     senderLoc.WardID,
			},
			AddressTo: GoShipAddress{
				Name:     fmt.Sprintf("%s %s", order.ShippingFirstName, order.ShippingLastName),
				Phone:    order.CustomerPhone,
				Street:   order.ShippingAddress1,
				City:     receiverLoc.StateID, // 👈 Map StateID vào City
				District: receiverLoc.DistrictID,
				Ward:     receiverLoc.WardID,
			},
			Parcel: GoShipParcel{
				Cod:      codAmount,
				Amount:   int64(order.GrandTotal),
				Weight:   weightInt,
				Width:    10,
				Height:   10,
				Length:   10,
				Metadata: note,
			},
		},
	}

	respBody, err := g.doRequest(c, http.MethodPost, "/shipments", payload)
	if err != nil {
		return "", 0, err
	}

	var result GoShipCreateOrderResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", 0, fmt.Errorf("unmarshal goship create order res error: %w", err)
	}

	if result.Code != 200 && result.Status != "success" {
		return "", 0, fmt.Errorf("goship create order failed: %s", result.Message)
	}

	trackingCode := result.TrackingNumber
	if trackingCode == "" || trackingCode == "NULL" {
		trackingCode = result.ID // Sử dụng mã đơn GoShip nếu chưa có tracking number ngay
	}

	return trackingCode, result.Fee, nil
}

// -----------------------------------------------------------------------------
// AUTO REGISTER IN INIT
// -----------------------------------------------------------------------------

func init() {
	utils.LogToFile("init GoShipProvider")
	provider := &GoShipProvider{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
	shipping.Register("goship", provider)
}

// GoShipCancelOrderResponse hứng response trả về từ API DELETE shipments
type GoShipCancelOrderResponse struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func (g *GoShipProvider) CancelShipment(c *gin.Context, trackingCode string) error {
	if trackingCode == "" {
		return fmt.Errorf("mã vận đơn (tracking code) không được để rỗng")
	}

	// Endpoint GoShip V2: DELETE /shipments/{id}
	endpoint := fmt.Sprintf("/shipments/%s", trackingCode)

	respBody, err := g.doRequest(c, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("lỗi kết nối tới GoShip API: %w", err)
	}

	var result GoShipCancelOrderResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("lỗi unmarshal response xóa vận đơn GoShip: %w", err)
	}

	// Kiểm tra phản hồi (Thành công khi code = 200)
	if result.Code != 200 || result.Status != "success" {
		errMsg := result.Message
		if errMsg == "" {
			errMsg = fmt.Sprintf("%v", result.Data)
		}
		return fmt.Errorf("xóa vận đơn GoShip thất bại: %s", errMsg)
	}

	return nil
}
