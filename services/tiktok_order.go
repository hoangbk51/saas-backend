package services

import (
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Map trạng thái TikTok sang order_status_id của Web
func MapTikTokStatusToOrderID(status string) int {
	switch status {
	case "UNPAID":
		return 1 // Chờ thanh toán
	case "AWAITING_SHIPMENT":
		return 2 // Đang xử lý / Chờ giao
	case "AWAITING_COLLECTION", "IN_TRANSIT":
		return 3 // Đang vận chuyển
	case "DELIVERED", "COMPLETED":
		return 4 // Hoàn thành
	case "CANCELLED":
		return 5 // Đã hủy
	default:
		return 1
	}
}

// SaveOrUpdateTikTokOrder ghi/cập nhật 1 đơn hàng vào CSDL
func SaveOrUpdateTikTokOrder(tx *sqlx.Tx, userID uint64, channelID int, ttOrder models.TikTokOrder) error {
	var existingOrderID uint64
	checkQuery := "SELECT `id` FROM `orders` WHERE `order_number` = ? LIMIT 1"
	err := tx.Get(&existingOrderID, checkQuery, ttOrder.ID)

	var safeCreatedAt time.Time
	if ttOrder.CreateTime > 0 {
		safeCreatedAt = time.Unix(ttOrder.CreateTime, 0)
	} else {
		safeCreatedAt = time.Now()
	}

	// Nếu đơn đã tồn tại -> Cập nhật trạng thái và mã vận đơn
	if err == nil && existingOrderID > 0 {
		updateQuery := "UPDATE `orders` SET `order_status_id` = ?, `tracking_id` = ?, `updated_at` = NOW() WHERE `id` = ?"
		_, err = tx.Exec(updateQuery, MapTikTokStatusToOrderID(ttOrder.Status), ttOrder.TrackingNumber, existingOrderID)
		return err
	}

	// Insert đơn mới
	itemCount := len(ttOrder.ItemList)
	totalQty := 0
	for _, item := range ttOrder.ItemList {
		totalQty += item.Quantity
	}

	orderQuery := "INSERT INTO `orders` (" +
		"`order_number`, `channel_id`, `email`, `shipping_first_name`, `shipping_address1`," +
		"`item_count`, `quantity`, `total`, `grand_total`, `tracking_id`, `payment_method_code`," +
		"`order_status_id`, `payment_status`, `delivery_status`, `created_at`, `updated_at`" +
		") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, NOW())"

	res, err := tx.Exec(orderQuery,
		ttOrder.ID, channelID, ttOrder.BuyerEmail, "Khách hàng TikTok", "Địa chỉ TikTok Shop",
		itemCount, totalQty, ttOrder.Payment.TotalAmount, ttOrder.Payment.TotalAmount,
		ttOrder.TrackingNumber, "TIKTOK_PAY", MapTikTokStatusToOrderID(ttOrder.Status),
		safeCreatedAt,
	)
	if err != nil {
		return fmt.Errorf("lỗi insert orders TikTok: %w", err)
	}

	orderID, _ := res.LastInsertId()

	// Insert order_items (đã bọc backticks chuẩn cho tên cột `option`)
	itemQuery := "INSERT INTO `order_items` (" +
		"`order_id`, `product_id`, `item_description`, `quantity`, `unit_price`, `option`, `created_at`, `updated_at`" +
		") VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())"

	for _, item := range ttOrder.ItemList {
		var mappedVariant struct {
			ProductID uint64 `db:"product_id"`
			VariantID uint64 `db:"id"`
		}

		itemPriceFloat, err := strconv.ParseFloat(item.Price, 64)
		if err != nil {
			itemPriceFloat = 0.0
		}

		findSKUQuery := "SELECT `product_id`, `id` FROM `product_variants` WHERE `sku` = ? LIMIT 1"
		err = tx.Get(&mappedVariant, findSKUQuery, item.SKUID)

		finalProductID := mappedVariant.ProductID
		if err != nil || finalProductID == 0 {
			finalProductID = 1 // Fallback Product ID mặc định
		}

		optionJSON := fmt.Sprintf(`{"sku_id":"%s","variant_id":%d}`, item.SKUID, mappedVariant.VariantID)

		_, err = tx.Exec(itemQuery, orderID, finalProductID, item.SKUName, item.Quantity, itemPriceFloat, optionJSON)
		if err != nil {
			return fmt.Errorf("lỗi insert order_items (SKU: %s): %w", item.SKUID, err)
		}
	}

	return nil
}

// FetchTikTokOrdersFromAPI thực hiện 2 bước gọi API (Search -> Detail) để lấy danh sách chi tiết đơn hàng
func FetchTikTokOrdersFromAPI(c *gin.Context, mockBaseURL string, pageSize string, reqBody map[string]interface{}) ([]models.TikTokOrder, error) {
	headers := map[string]string{
		"x-tts-access-token": c.GetHeader("Authorization"),
	}

	if reqBody == nil {
		reqBody = make(map[string]interface{})
	}

	// BƯỚC 1: Gọi POST /tiktok/order/search
	searchURL := fmt.Sprintf("%s/tiktok/order/search?page_size=%s", mockBaseURL, pageSize)
	respBytes, err := utils.RequestMock(c, "POST", searchURL, headers, reqBody)
	if err != nil && len(respBytes) == 0 {
		return nil, fmt.Errorf("lỗi kết nối TikTok Search API: %w", err)
	}

	// Log trực tiếp chuỗi raw bytes trả về từ server (Tránh crash do json.Marshal lại)
	utils.LogToFile(string(respBytes))

	var searchRes models.TikTokSearchOrdersResponse
	if err := json.Unmarshal(respBytes, &searchRes); err != nil {
		return nil, fmt.Errorf("lỗi parse JSON từ TikTok Search API: %w", err)
	}

	if searchRes.Code != 0 {
		return nil, fmt.Errorf("TikTok Search API trả về lỗi: %s", searchRes.Message)
	}

	if len(searchRes.Data.Orders) == 0 {
		return []models.TikTokOrder{}, nil
	}

	// Lấy danh sách Order IDs
	var orderIDs []string
	for _, ord := range searchRes.Data.Orders {
		if ord.ID != "" {
			orderIDs = append(orderIDs, ord.ID)
		}
	}
	utils.LogToFile(orderIDs)
	// CHẶN TẠI ĐÂY: Nếu không lấy được ID nào từ Bước 1 thì dừng luôn
	if len(orderIDs) == 0 {
		utils.LogToFile("CẢNH BÁO: Không bóc tách được order_id nào từ Search API")
		return []models.TikTokOrder{}, nil
	}

	// BƯỚC 2: Gọi API Detail lấy chi tiết đơn hàng
	// Ghép các ID thành chuỗi query: ids=576829103829102
	idsParam := strings.Join(orderIDs, ",")
	detailURL := fmt.Sprintf("%s/tiktok/order/detail?order_ids=%s", mockBaseURL, idsParam)
	utils.LogToFile(detailURL)

	detailRespBytes, err := utils.RequestMock(c, "GET", detailURL, headers, nil)
	if err != nil {
		return nil, fmt.Errorf("lỗi kết nối TikTok Detail API: %w", err)
	}
	utils.LogToFile("12323")

	// Ghi log RAW JSON của Detail API ra để kiểm tra
	utils.LogToFile(string(detailRespBytes))

	var detailRes models.TikTokOrderDetailResponse
	utils.LogToFile("abcd")
	if err := json.Unmarshal(detailRespBytes, &detailRes); err != nil {
		utils.LogToFile("lỗi parse dữ liệu: " + err.Error())
		return nil, fmt.Errorf("lỗi parse dữ liệu chi tiết đơn từ TikTok: %w", err)
	}

	// Log dữ liệu đã parse thành công ra định dạng JSON để kiểm tra
	parsedJSON, _ := json.Marshal(detailRes.Data.Orders)
	utils.LogToFile("Dữ liệu sau khi parse: " + string(parsedJSON))

	return detailRes.Data.Orders, nil
}
