package services

import (
	"encoding/json"
	"fmt"
	"go-saas/utils"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type SyncOrdersRequest struct {
	ChannelID uint64 `json:"channel_id" binding:"required"` // ID shop kết nối
	DayCount  int    `json:"day_count"`                     // Số ngày quá khứ cần kéo (mặc định 15 ngày)
}

type ChannelCredential struct {
	ID          uint64 `db:"id"`
	UserID      uint64 `db:"user_id"`
	ChannelName string `db:"channel_name"` // "shopee" hoặc "tiktok"
	PartnerID   string `db:"partner_id"`
	ShopID      string `db:"shop_id"`
	AccessToken string `db:"access_token"`
	BaseURL     string `db:"base_url"`
}

// getChannelCredentials lấy thông tin API Credential từ bảng channel_accounts/user_channels
func getChannelCredentials(db *sqlx.DB, channelID uint64, userID uint64) (*ChannelCredential, error) {
	var cred ChannelCredential
	query := `
		SELECT id, user_id, channel_name, partner_id, shop_id, access_token, base_url 
		FROM user_channels 
		WHERE id = ? AND user_id = ? AND status = 'ACTIVE' 
		LIMIT 1`

	utils.LogSQL(query, channelID, userID)
	err := db.Get(&cred, query, channelID, userID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy kênh kết nối hợp lệ: %w", err)
	}

	return &cred, nil
}

// SyncOrdersFromChannel kéo danh sách đơn từ Shopee/TikTok về lưu DB
func SyncOrdersFromChannel(c *gin.Context, req SyncOrdersRequest) (int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, fmt.Errorf("lỗi kết nối database: %w", err)
	}

	userID := c.GetUint64("user_id")
	if req.DayCount <= 0 {
		req.DayCount = 15
	}

	//channel, err := getChannelCredentials(db, req.ChannelID, userID)
	//if err != nil {
	//	return 0, err
	//}

	mockBaseURL := getShopeeMockBase(c)
	if mockBaseURL == "" {
		//	mockBaseURL = channel.BaseURL
	}

	ordersFromChannel, err := fetchOrdersFromShopeeAPI(
		mockBaseURL,
		"", //channel.PartnerID,
		"", //channel.ShopID,
		req.DayCount,
	)
	if err != nil {
		return 0, fmt.Errorf("lỗi gọi API Shopee: %w", err)
	}

	if len(ordersFromChannel) == 0 {
		return 0, nil
	}

	tx, err := db.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	syncedCount := 0
	utils.LogToFile(ordersFromChannel)

	for _, chOrder := range ordersFromChannel {
		chOrder.ChannelID = req.ChannelID
		utils.LogToFile(chOrder)

		var existingOrderID uint64
		// Khớp mã đơn sàn với cột order_number trong DB
		checkQuery := `SELECT id FROM orders WHERE order_number = ? LIMIT 1`
		err := tx.Get(&existingOrderID, checkQuery, chOrder.OrderSN)
		utils.LogToFile(err)

		if err == nil && existingOrderID > 0 {
			utils.LogToFile("if")

			// Đơn đã tồn tại -> Chỉ update order_status_id và tracking_id
			updateQuery := `
				UPDATE orders 
				SET order_status_id = ?, tracking_id = ?, updated_at = NOW() 
				WHERE id = ?`
			utils.LogSQL(updateQuery, mapShopeeStatusToOrderID(chOrder.Status), chOrder.TrackingNumber, existingOrderID)
			_, _ = tx.Exec(updateQuery, mapShopeeStatusToOrderID(chOrder.Status), chOrder.TrackingNumber, existingOrderID)
		} else {
			utils.LogToFile("insertOrderRecord")

			// Đơn mới -> Chèn đơn mới (KHÔNG TRỪ KHO)
			err := insertOrderRecord(tx, userID, chOrder)
			utils.LogToFile(err)

			if err != nil {
				continue
			}
			utils.LogToFile(syncedCount)
			syncedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return syncedCount, nil
}

func insertOrderRecord(tx *sqlx.Tx, userID uint64, chOrder ChannelOrderDTO) error {
	itemCount := len(chOrder.Items)
	totalQuantity := 0
	for _, it := range chOrder.Items {
		totalQuantity += it.Quantity
	}

	orderQuery := `
		INSERT INTO orders (
			order_number, channel_id, customer_phone, 
			shipping_first_name, shipping_address1,
			item_count, quantity, total, grand_total, 
			tracking_id, payment_method_code, order_status_id, 
			payment_status, delivery_status, created_at, updated_at
		) VALUES (
			?, ?, ?, 
			?, ?, 
			?, ?, ?, ?, 
			?, ?, ?, 
			1, 1, ?, NOW()
		)`

	res, err := tx.Exec(orderQuery,
		chOrder.OrderSN, chOrder.ChannelID, chOrder.CustomerPhone,
		chOrder.CustomerName, "Địa chỉ từ Sàn",
		itemCount, totalQuantity, chOrder.TotalAmount, chOrder.TotalAmount,
		chOrder.TrackingNumber, chOrder.PaymentMethod, mapShopeeStatusToOrderID(chOrder.Status),
		chOrder.CreatedAt,
	)
	if err != nil {
		utils.LogToFile(err)

		return fmt.Errorf("lỗi insert orders: %w", err)
	}

	orderID, _ := res.LastInsertId()
	// 2. Insert vào bảng order_items (Đã bọc double quotes đúng cú pháp Go)
	itemQuery := "INSERT INTO `order_items` (" +
		"`order_id`, `product_id`, `item_description`, `quantity`, `unit_price`, `option`, `created_at`, `updated_at`" +
		") VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())"

	for _, item := range chOrder.Items {
		utils.LogToFile(item)
		/*
			var mappedVariant struct {
				ProductID uint64 `db:"product_id"`
				VariantID uint64 `db:"id"`
			}

			findSKUQuery := `
			SELECT product_id, id
			FROM product_variants
			WHERE user_id = ? AND sku = ?
			LIMIT 1`

			err := tx.Get(&mappedVariant, findSKUQuery, userID, item.SKU)
			if err == nil {
				item.ProductID = mappedVariant.ProductID
				item.VariantID = mappedVariant.VariantID
			}
		*/
		var finalProductID interface{} = item.ProductID
		if item.ProductID == 0 {
			finalProductID = nil
		}

		optionJSON := fmt.Sprintf(`{"sku":"%s","variant_id":%d}`, item.SKU, item.VariantID)

		utils.LogSQL(itemQuery, orderID, finalProductID, item.ProductName, item.Quantity, item.Price, optionJSON)

		_, err = tx.Exec(itemQuery, orderID, finalProductID, item.ProductName, item.Quantity, item.Price, optionJSON)
		if err != nil {
			return fmt.Errorf("lỗi insert order_items (SKU: %s): %w", item.SKU, err)
		}
	}

	return nil
}

type ShopeeOrderListResponse struct {
	Error    string `json:"error"`
	Message  string `json:"message"`
	Response struct {
		More       bool   `json:"more"`
		NextCursor string `json:"next_cursor"`
		OrderList  []struct {
			OrderSN string `json:"order_sn"`
		} `json:"order_list"`
	} `json:"response"`
}

type FlexFloat64 float64

func (f *FlexFloat64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*f = 0
		return nil
	}

	// Nếu dữ liệu là String (bắt đầu và kết thúc bằng dấu " )
	if data[0] == '"' && data[len(data)-1] == '"' {
		strVal := string(data[1 : len(data)-1])
		if strVal == "" {
			*f = 0
			return nil
		}
		val, err := strconv.ParseFloat(strVal, 64)
		if err != nil {
			return fmt.Errorf("không thể parse string %s sang float64: %w", strVal, err)
		}
		*f = FlexFloat64(val)
		return nil
	}

	// Nếu dữ liệu là Number bình thường
	var val float64
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	*f = FlexFloat64(val)
	return nil
}

type ShopeeOrderDetailItem struct {
	ItemID        int64       `json:"item_id"`
	ItemName      string      `json:"item_name"`
	ItemSKU       string      `json:"item_sku"`
	ModelID       int64       `json:"model_id"`
	ModelSKU      string      `json:"model_sku"`
	ModelName     string      `json:"model_name"`
	ModelQuantity int         `json:"model_quantity_purchased"`
	ModelPrice    FlexFloat64 `json:"model_discounted_price"` // Đổi float64 -> FlexFloat64
}

type ShopeeOrderDetail struct {
	OrderSN          string      `json:"order_sn"`
	OrderStatus      string      `json:"order_status"`
	TotalAmount      FlexFloat64 `json:"total_amount"` // Đổi float64 -> FlexFloat64
	PaymentMethod    string      `json:"payment_method"`
	TrackingNo       string      `json:"tracking_number"`
	RecipientAddress struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"recipient_address"`
	ItemList   []ShopeeOrderDetailItem `json:"item_list"`
	CreateTime int64                   `json:"create_time"`
}

type ShopeeOrderDetailResponse struct {
	Error    string `json:"error"`
	Message  string `json:"message"`
	Response struct {
		OrderList []ShopeeOrderDetail `json:"order_list"`
	} `json:"response"`
}

// Struct chuẩn hóa nội bộ trả về cho Service lưu DB
type ChannelOrderDTO struct {
	ChannelID      uint64
	OrderSN        string
	CustomerName   string
	CustomerPhone  string
	TotalAmount    float64
	Status         string
	PaymentMethod  string
	TrackingNumber string
	CreatedAt      time.Time
	Items          []ChannelOrderItemDTO
}

type ChannelOrderItemDTO struct {
	ProductID   uint64 // Thêm trường này
	VariantID   uint64 // Thêm trường này
	SKU         string
	ProductName string
	Price       float64
	Quantity    int
}

// fetchOrdersFromShopeeAPI thực hiện gọi Mock API lấy danh sách và chi tiết đơn hàng
func fetchOrdersFromShopeeAPI(baseURL, partnerID, shopID string, dayCount int) ([]ChannelOrderDTO, error) {
	timeFrom := time.Now().AddDate(0, 0, -dayCount).Unix()
	timeTo := time.Now().Unix()

	// BƯỚC 1: Gọi API get_order_list lấy danh sách mã order_sn
	listURL := fmt.Sprintf(
		"%s/api/v2/shopee/order/get_order_list?partner_id=%s&shop_id=%s&time_range_field=create_time&time_from=%d&time_to=%d&page_size=50",
		baseURL, partnerID, shopID, timeFrom, timeTo,
	)

	resp, err := http.Get(listURL)
	if err != nil {
		return nil, fmt.Errorf("lỗi kết nối API Shopee List: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var listRes ShopeeOrderListResponse
	if err := json.Unmarshal(bodyBytes, &listRes); err != nil {
		return nil, fmt.Errorf("lỗi parse json order list: %w", err)
	}

	if len(listRes.Response.OrderList) == 0 {
		return []ChannelOrderDTO{}, nil
	}

	// Gom danh sách mã order_sn
	var orderSNs []string
	for _, item := range listRes.Response.OrderList {
		orderSNs = append(orderSNs, item.OrderSN)
	}
	utils.LogToFile(orderSNs)
	// BƯỚC 2: Gọi API get_order_detail lấy đầy đủ thông tin chi tiết
	snListStr := strings.Join(orderSNs, ",")
	// Yêu cầu Shopee trả thêm mảng item_list và recipient_address
	optionalFields := "item_list,recipient_address"

	detailURL := fmt.Sprintf(
		"%s/api/v2/shopee/order/get_order_detail?order_sn_list=%s&response_optional_fields=%s",
		baseURL, snListStr, optionalFields,
	)

	respDetail, err := http.Get(detailURL)
	if err != nil {
		return nil, fmt.Errorf("lỗi kết nối API Shopee Detail: %w", err)
	}
	defer respDetail.Body.Close()

	detailBytes, _ := io.ReadAll(respDetail.Body)
	var detailRes ShopeeOrderDetailResponse
	if err := json.Unmarshal(detailBytes, &detailRes); err != nil {
		return nil, fmt.Errorf("lỗi parse json order detail: %w", err)
	}
	utils.LogToFile(detailRes)
	// BƯỚC 3: Map dữ liệu từ Shopee Response sang ChannelOrderDTO chuẩn hóa
	var result []ChannelOrderDTO
	for _, raw := range detailRes.Response.OrderList {
		var createdAt time.Time
		if raw.CreateTime > 0 {
			createdAt = time.Unix(raw.CreateTime, 0)
		} else {
			createdAt = time.Now()
		}
		utils.LogToFile("raw")

		utils.LogToFile(raw)

		orderDTO := ChannelOrderDTO{
			OrderSN:        raw.OrderSN,
			CustomerName:   raw.RecipientAddress.Name,
			CustomerPhone:  raw.RecipientAddress.Phone,
			TotalAmount:    float64(raw.TotalAmount), // Ép kiểu FlexFloat64 -> float64
			Status:         mapShopeeStatusToWeb(raw.OrderStatus),
			PaymentMethod:  raw.PaymentMethod,
			TrackingNumber: raw.TrackingNo,
			CreatedAt:      createdAt,
		}
		utils.LogToFile(raw.ItemList)

		for _, item := range raw.ItemList {
			sku := item.ModelSKU
			if sku == "" {
				sku = item.ItemSKU
			}
			utils.LogToFile("item")

			utils.LogToFile(item)
			utils.LogToFile(item.ItemID)
			utils.LogToFile(uint64(item.ItemID))

			orderDTO.Items = append(orderDTO.Items, ChannelOrderItemDTO{
				SKU:         sku,
				ProductName: item.ItemName + " - " + item.ModelName,
				Price:       float64(item.ModelPrice), // Ép kiểu FlexFloat64 -> float64
				Quantity:    item.ModelQuantity,
				ProductID:   uint64(item.ItemID),
			})
			utils.LogToFile(orderDTO.Items)

		}

		result = append(result, orderDTO)
	}
	utils.LogToFile(result)

	return result, nil
}

// Helper chuẩn hóa trạng thái từ Shopee về Enum DB Web SaaS
func mapShopeeStatusToWeb(shopeeStatus string) string {
	switch shopeeStatus {
	case "UNPAID":
		return "PENDING"
	case "READY_TO_SHIP":
		return "PROCESSING"
	case "PROCESSED", "SHIPPED":
		return "SHIPPED"
	case "COMPLETED":
		return "DELIVERED"
	case "CANCELLED":
		return "CANCELLED"
	case "IN_CANCEL":
		return "CANCEL_REQUESTED"
	default:
		return "PENDING"
	}
}

func mapShopeeStatusToOrderID(status string) int {
	switch status {
	case "PENDING", "UNPAID":
		return 1 // Chờ thanh toán / Chờ xử lý
	case "PROCESSING", "READY_TO_SHIP":
		return 2 // Đang xử lý
	case "SHIPPED", "PROCESSED":
		return 3 // Đang giao
	case "DELIVERED", "COMPLETED":
		return 4 // Hoàn thành
	case "CANCELLED":
		return 5 // Đã hủy
	default:
		return 1
	}
}
