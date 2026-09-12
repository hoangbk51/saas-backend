package providers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"go-saas/models"
	"go-saas/shipping"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type StoreInfo struct {
	GroupAddressID int `json:"groupaddressId"`
	CusID          int `json:"cusId"`
}

type ViettelStoreResponse struct {
	Status int         `json:"status"`
	Data   []StoreInfo `json:"data"`
}

type ViettelShipProvider struct {
	BaseURL             string
	tokenCache          string
	groupAddressIDCache int
	cusIDCache          int
	tokenExp            time.Time
	mu                  sync.Mutex
}

type ViettelPriceRequest struct {
	SenderProvince   int `json:"SENDER_PROVINCE"`
	SenderDistrict   int `json:"SENDER_DISTRICT"`
	ReceiverProvince int `json:"RECEIVER_PROVINCE"`
	ReceiverDistrict int `json:"RECEIVER_DISTRICT"`
	ProductWeight    int `json:"PRODUCT_WEIGHT"`
	ProductPrice     int `json:"PRODUCT_PRICE"`
	MoneyCollection  int `json:"MONEY_COLLECTION"`
	Type             int `json:"TYPE"`
}

type ViettelServiceResponse struct {
	MaDVChinh string  `json:"MA_DV_CHINH"`
	TenDichVu string  `json:"TEN_DICHVU"`
	GiaCuoc   float64 `json:"GIA_CUOC"`
}

func init() {
	utils.LogToFile("init ViettelShipProvider")
	provider := &ViettelShipProvider{
		BaseURL: "https://partner.viettelpost.vn/v2",
	}
	shipping.Register("viettelpost", provider)
}

func (p *ViettelShipProvider) GetName() string {
	return "Viettel Post"
}

// Map Tỉnh/Huyện người nhận qua DB Center (*sql.DB)
func (p *ViettelShipProvider) getMappingIDs(c *gin.Context, shippingDistrictID *int64, shippingStateID *int64) (int, int, error) {
	dbCenter, err := utils.GetCentralDB()
	if err != nil {
		return 0, 0, fmt.Errorf("lỗi kết nối DB Center: %v", err)
	}

	ctx := c.Request.Context()
	var receiverProvinceID int
	var receiverDistrictID int

	// 🟢 Sửa: Kiểm tra != nil và dùng *cart.ShippingStateID
	if shippingStateID != nil {
		query := "SELECT PROVINCE_ID FROM viettel_provinces WHERE app_state_id = ? LIMIT 1"
		err := dbCenter.QueryRowContext(ctx, query, shippingStateID).Scan(&receiverProvinceID)
		if err != nil && err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[ViettelPost] Lỗi query PROVINCE_ID: %v", err))
		}
	}

	// 🟢 Sửa: Kiểm tra != nil và dùng *cart.ShippingDistrictID
	if shippingDistrictID != nil {
		query := "SELECT DISTRICT_ID FROM viettel_districts WHERE app_district_id = ? LIMIT 1"
		err := dbCenter.QueryRowContext(ctx, query, shippingDistrictID).Scan(&receiverDistrictID)
		if err != nil && err != sql.ErrNoRows {
			utils.LogToFile(fmt.Sprintf("[ViettelPost] Lỗi query DISTRICT_ID: %v", err))
		}
	}

	return receiverProvinceID, receiverDistrictID, nil
}

// CalculateFee trả về phí gói SCN hoặc gói đầu tiên (hoặc fallback 25000)
func (p *ViettelShipProvider) CalculateFee(c *gin.Context, cart *models.Cart) (float64, error) {
	services, err := p.GetServices(c, cart)
	if err != nil {
		utils.LogToFile(fmt.Sprintf("[ViettelPost] Fallback cước 25,000 do lỗi: %v", err))
		return 25000, nil
	}

	for _, service := range services {
		if service.Code == "SCN" {
			return service.Value, nil
		}
	}

	if len(services) > 0 {
		return services[0].Value, nil
	}

	return 25000, nil
}

type ViettelCreateOrderPayload struct {
	OrderNumber     string `json:"ORDER_NUMBER"`
	GroupAddressID  int    `json:"GROUPADDRESS_ID"`
	CusName         string `json:"RECEIVER_FULLNAME"`
	CusPhone        string `json:"RECEIVER_PHONE"`
	CusAddress      string `json:"RECEIVER_ADDRESS"`
	CusProvince     int    `json:"RECEIVER_PROVINCE"`
	CusDistrict     int    `json:"RECEIVER_DISTRICT"`
	ProductWeight   int    `json:"PRODUCT_WEIGHT"`
	ProductPrice    int    `json:"PRODUCT_PRICE"`
	MoneyCollection int    `json:"MONEY_COLLECTION"`
	OrderNote       string `json:"ORDER_NOTE"`
	SenderProvince  int
	SenderDistrict  int
}

type ViettelCreateOrderRequest struct {
	OrderNumber      string `json:"ORDER_NUMBER"`
	GroupAddressID   int    `json:"GROUPADDRESS_ID"`
	CusID            int    `json:"CUS_ID"`
	ReceiverFullname string `json:"RECEIVER_FULLNAME"`
	ReceiverAddress  string `json:"RECEIVER_ADDRESS"`
	ReceiverPhone    string `json:"RECEIVER_PHONE"`
	ReceiverProvince int    `json:"RECEIVER_PROVINCE"`
	ReceiverDistrict int    `json:"RECEIVER_DISTRICT"`
	ReceiverWard     int    `json:"RECEIVER_WARD"`
	ProductName      string `json:"PRODUCT_NAME"`
	ProductWeight    int    `json:"PRODUCT_WEIGHT"`
	ProductQuantity  int    `json:"PRODUCT_QUANTITY"`
	ProductPrice     int    `json:"PRODUCT_PRICE"`
	MoneyCollection  int    `json:"MONEY_COLLECTION"`
	OrderPayment     int    `json:"ORDER_PAYMENT"` // 1: Người gửi trả, 2: Người nhận trả
	OrderService     string `json:"ORDER_SERVICE"` // VCN, PHS, VBS...
	ProductType      string `json:"PRODUCT_TYPE"`  // HH: Hàng hóa, TL: Tài liệu
	ProductLength    int    `json:"PRODUCT_LENGTH,omitempty"`
	ProductWidth     int    `json:"PRODUCT_WIDTH,omitempty"`
	ProductHeight    int    `json:"PRODUCT_HEIGHT,omitempty"`
	OrderNote        string `json:"ORDER_NOTE,omitempty"`
}

type ViettelCreateOrderResponse struct {
	Status  int    `json:"status"`
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    struct {
		OrderNumber     string  `json:"ORDER_NUMBER"` // Mã vận đơn ViettelPost
		MoneyCollection float64 `json:"MONEY_COLLECTION"`
		MoneyTotal      float64 `json:"MONEY_TOTAL"` // Tổng cước
		MoneyFee        float64 `json:"MONEY_FEE"`
	} `json:"data"`
}

// CreateOrder tạo đơn hàng vận chuyển ViettelPost
func (p *ViettelShipProvider) CreateOrder(c *gin.Context, order *models.Order, note string) (string, float64, error) {
	// 1. Lấy Credential (Token, groupAddressID, cusID) từ Plugin Settings
	token, groupAddressID, cusID, errCred := p.GetViettelCredentials(c)
	if errCred != nil {
		return "", 0, fmt.Errorf("lỗi xác thực ViettelPost: %w", errCred)
	}

	// 2. Mapping Tỉnh/Huyện người nhận từ DB Center
	receiverProvinceID, receiverDistrictID, err := p.getMappingIDs(c, order.ShippingCity, order.ShippingState)
	if err != nil || receiverProvinceID == 0 || receiverDistrictID == 0 {
		return "", 0, fmt.Errorf("không tìm thấy mã Tỉnh/Huyện giao hàng hợp lệ cho ViettelPost")
	}

	// 3. Lấy cấu hình dịch vụ & loại hình thanh toán từ Plugin Setting
	serviceCode := "PHS"
	if order.ShippingMethodSubCode != "" {
		serviceCode = order.ShippingMethodSubCode
	}

	paymentTypeStr, _ := utils.GetPluginSetting(c, "viettelpost", "viettel_post_payment_type")
	paymentType := utils.ParseIntOrDefault(paymentTypeStr, 1) // Default: 1 - Shop trả phí

	// 4. Chuẩn bị Payload gửi ViettelPost API
	payload := ViettelCreateOrderRequest{
		OrderNumber:      fmt.Sprintf("%s", order.OrderNumber),
		GroupAddressID:   groupAddressID,
		CusID:            cusID,
		ReceiverFullname: fmt.Sprintf("%s %s", order.ShippingFirstName, order.ShippingLastName),
		ReceiverAddress:  order.ShippingAddress1,
		ReceiverPhone:    order.CustomerPhone,
		ReceiverProvince: receiverProvinceID,
		ReceiverDistrict: receiverDistrictID,
		ReceiverWard:     981,
		ProductName:      "Đơn hàng #" + order.OrderNumber,
		ProductWeight:    int(order.TotalWeight),
		ProductQuantity:  1,
		ProductPrice:     int(order.GrandTotal),
		MoneyCollection:  int(order.GrandTotal), // Thu hộ COD
		OrderPayment:     paymentType,
		OrderService:     serviceCode,
		ProductType:      "HH",
		OrderNote:        note,
		ProductLength:    10,
		ProductHeight:    10,
		ProductWidth:     10,
	}

	utils.LogToFile(payload)
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", 0, fmt.Errorf("lỗi marshal JSON payload: %w", err)
	}

	// 5. Gọi API ViettelPost CreateOrder
	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", p.BaseURL+"/order/createOrder", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Token", token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("không thể kết nối API ViettelPost: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	// 6. Parse Response từ ViettelPost
	var apiResp ViettelCreateOrderResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		utils.LogToFile(err)
		return "", 0, fmt.Errorf("lỗi đọc JSON response: %w", err)
	}

	if apiResp.Status != 200 || apiResp.Error {
		utils.LogToFile(apiResp)
		return "", 0, fmt.Errorf("lỗi từ ViettelPost: %s", apiResp.Message)
	}

	// Trả về Mã vận đơn (ORDER_NUMBER) và Tổng phí giao hàng (MONEY_TOTAL)
	return apiResp.Data.OrderNumber, apiResp.Data.MoneyTotal, nil
}

// GetServices lấy danh sách gói cước và giá từ API ViettelPost
func (p *ViettelShipProvider) GetServices(c *gin.Context, cart *models.Cart) ([]models.ShippingService, error) {
	utils.LogToFile("[ViettelPost] Calculating shipping rate")

	// 1. Lấy thông tin Tỉnh/Huyện gửi từ Plugin Setting
	//senderProvinceStr, _ := utils.GetPluginSetting(c, "viettelpost", "sender_province_id")
	//senderDistrictStr, _ := utils.GetPluginSetting(c, "viettelpost", "sender_district_id")

	senderProvinceID := 1 // utils.ParseIntOrDefault(senderProvinceStr, 0)
	senderDistrictID := 1 //utils.ParseIntOrDefault(senderDistrictStr, 0)

	utils.LogToFile(fmt.Sprintf("[ViettelPost] Sender Province: %d, District: %d", senderProvinceID, senderDistrictID))

	if senderProvinceID == 0 || senderDistrictID == 0 {
		return nil, fmt.Errorf("chưa cấu hình địa chỉ gửi ViettelPost trong Plugin Settings")
	}

	// 2. Lấy ID Tỉnh/Huyện nhận từ DB Center
	receiverProvinceID, receiverDistrictID, err := p.getMappingIDs(c, cart.ShippingDistrictID, cart.ShippingStateID)
	if err != nil || receiverProvinceID == 0 || receiverDistrictID == 0 {
		return nil, fmt.Errorf("không tìm thấy mapping địa chỉ ViettelPost trong DB Center")
	}

	// 3. Chuẩn bị Request Payload
	weight := int(cart.ShippingWeight)
	if weight <= 0 {
		weight = 1000 // Mặc định 1000g
	}

	totalAmount := int(cart.Total)

	payload := ViettelPriceRequest{
		SenderProvince:   senderProvinceID,
		SenderDistrict:   senderDistrictID,
		ReceiverProvince: receiverProvinceID,
		ReceiverDistrict: receiverDistrictID,
		ProductWeight:    weight,
		ProductPrice:     totalAmount,
		MoneyCollection:  totalAmount,
		Type:             1,
	}

	utils.LogToFile(fmt.Sprintf("[ViettelPost] Payload: %+v", payload))

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// 4. Gọi API ViettelPost GetPriceAll
	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", p.BaseURL+"/order/getPriceAll", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Lấy Token (nếu lỗi vẫn cho phép chạy tiếp dạng Public Request)
	token, _, _, errToken := p.GetViettelCredentials(c)
	if errToken != nil {
		utils.LogToFile(fmt.Sprintf("[ViettelPost] Warning: Không lấy được Token (%v), dùng public mode", errToken))
	}
	if token != "" {
		req.Header.Set("Token", token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.LogToFile(fmt.Sprintf("[ViettelPost] HTTP Error: %v", err))
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	utils.LogToFile(fmt.Sprintf("[ViettelPost] Response Raw: %s", string(body)))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("viettel API lỗi HTTP status: %d", resp.StatusCode)
	}

	var services []ViettelServiceResponse
	if err := json.Unmarshal(body, &services); err != nil || len(services) == 0 {
		return nil, fmt.Errorf("không lấy được danh sách dịch vụ ViettelPost")
	}

	// 5. Convert ra danh sách dịch vụ
	resultServices := make([]models.ShippingService, 0, len(services))
	for _, s := range services {
		name := s.TenDichVu
		if name == "" {
			name = "Viettel " + s.MaDVChinh
		}

		resultServices = append(resultServices, models.ShippingService{
			Code:  s.MaDVChinh,
			Name:  name,
			Value: s.GiaCuoc,
		})
	}

	// Sắp xếp danh sách dịch vụ theo giá (Value) từ thấp đến cao
	sort.Slice(resultServices, func(i, j int) bool {
		return resultServices[i].Value < resultServices[j].Value
	})

	return resultServices, nil
}

func (p *ViettelShipProvider) GetViettelCredentials(c *gin.Context) (token string, groupAddressID int, cusID int, err error) {
	// 1. Đọc Token, Expiry và Store Info từ Plugin Setting của Tenant
	token, _ = utils.GetPluginSetting(c, "viettelpost", "viettel_test_token")
	expiredAtStr, _ := utils.GetPluginSetting(c, "viettelpost", "viettel_test_token_expired_at")
	groupAddressStr, _ := utils.GetPluginSetting(c, "viettelpost", "viettel_test_group_address_id")
	cusIDStr, _ := utils.GetPluginSetting(c, "viettelpost", "viettel_test_cus_id")

	expiredAt := utils.ParseInt64OrDefault(expiredAtStr, 0)
	now := time.Now().Unix()

	// 2. Nếu đã có Token, chưa hết hạn (chừa 5 phút gia hạn) và có đủ Store IDs -> Trả về luôn
	if token != "" && expiredAt > (now+300) && groupAddressStr != "" && cusIDStr != "" {
		return token, utils.ParseIntOrDefault(groupAddressStr, 0), utils.ParseIntOrDefault(cusIDStr, 0), nil
	}

	// 3. Nếu Token hết hạn hoặc chưa có Store IDs -> Tự động Login & Lấy Kho mới
	username, _ := utils.GetPluginSetting(c, "viettelpost", "viettel_test_username")
	password, _ := utils.GetPluginSetting(c, "viettelpost", "viettel_test_password")
	utils.LogToFile(username)
	utils.LogToFile(password)
	if username == "" || password == "" {
		return "", 0, 0, fmt.Errorf("chưa cấu hình Tài khoản/Mật khẩu ViettelPost")
	}

	// Gọi API Đăng nhập ViettelPost
	newToken, errLogin := p.callApiLogin(c, username, password)
	if errLogin != nil {
		return "", 0, 0, errLogin
	}

	// Gọi API Lấy danh sách kho
	newGroupAddressID, newCusID, errInventory := p.callApiGetInventory(c, newToken)
	if errInventory != nil {
		return "", 0, 0, errInventory
	}

	// 4. Lưu ngược lại thông tin mới vào Plugin Setting cho Tenant này
	newExpiredAt := time.Now().Add(23 * time.Hour).Unix()

	_ = utils.SetPluginSetting(c, "viettelpost", "viettel_test_token", newToken)
	_ = utils.SetPluginSetting(c, "viettelpost", "viettel_test_token_expired_at", fmt.Sprintf("%d", newExpiredAt))
	_ = utils.SetPluginSetting(c, "viettelpost", "viettel_test_group_address_id", fmt.Sprintf("%d", newGroupAddressID))
	_ = utils.SetPluginSetting(c, "viettelpost", "viettel_test_cus_id", fmt.Sprintf("%d", newCusID))

	return newToken, newGroupAddressID, newCusID, nil
}

// callApiLogin thực hiện đăng nhập ViettelPost để lấy Token
func (p *ViettelShipProvider) callApiLogin(c *gin.Context, username, password string) (string, error) {
	loginPayload := map[string]string{
		"USERNAME": username,
		"PASSWORD": password,
	}

	jsonBytes, err := json.Marshal(loginPayload)
	if err != nil {
		return "", fmt.Errorf("lỗi marshal login payload: %w", err)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", p.BaseURL+"/user/login", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("lỗi kết nối API Login ViettelPost: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("lỗi đọc response login: %w", err)
	}

	var loginResult struct {
		Status int `json:"status"`
		Data   struct {
			Token string `json:"token"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &loginResult); err != nil {
		return "", fmt.Errorf("lỗi parse JSON response login: %w", err)
	}

	if loginResult.Status != 200 || loginResult.Data.Token == "" {
		return "", fmt.Errorf("đăng nhập ViettelPost thất bại: %s", loginResult.Message)
	}

	return loginResult.Data.Token, nil
}

// callApiGetInventory lấy groupAddressID và cusID của kho hàng đầu tiên
func (p *ViettelShipProvider) callApiGetInventory(c *gin.Context, token string) (groupAddressID int, cusID int, err error) {
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", p.BaseURL+"/user/listInventory", nil)
	if err != nil {
		return 0, 0, err
	}

	req.Header.Set("Token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("lỗi kết nối API listInventory: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("lỗi đọc response listInventory: %w", err)
	}

	var storeResp struct {
		Status int `json:"status"`
		Data   []struct {
			GroupAddressID int `json:"groupaddressId"`
			CusID          int `json:"cusId"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &storeResp); err != nil {
		return 0, 0, fmt.Errorf("lỗi parse JSON danh sách kho ViettelPost: %w", err)
	}

	if storeResp.Status != 200 || len(storeResp.Data) == 0 {
		return 0, 0, fmt.Errorf("tài khoản ViettelPost chưa tạo kho hàng (Inventory) hoặc bị lỗi: %s", storeResp.Message)
	}

	// Lấy thông tin kho mặc định (phần tử đầu tiên)
	firstStore := storeResp.Data[0]
	return firstStore.GroupAddressID, firstStore.CusID, nil
}

func (g *ViettelShipProvider) CancelShipment(c *gin.Context, trackingCode string) error {
	return nil
}
