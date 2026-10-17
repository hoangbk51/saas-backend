package services

import (
	"context"
	"encoding/json"
	"go-saas/models"
	"go-saas/utils"
	"net/http"
	"strconv"
	"strings"

	// Thay bằng import gin thực tế của bạn: "github.com/gin-gonic/gin"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func ProcessVirtualTryOnService(c *gin.Context) (interface{}, int, error) {
	// 1. Lấy DB Connection từ Context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": "Không thể kết nối CSDL: " + err.Error()}, http.StatusInternalServerError, err
	}
	utils.LogToFile("ProcessVirtualTryOnService")
	// 2. Lấy customerID từ Gin Context (từ middleware JWT/Auth)
	customerID := getCustomerIDFromContext(c)
	utils.LogToFile(customerID)

	if customerID == 0 {
		return gin.H{"error": "Không tìm thấy thông tin khách hàng (Chưa đăng nhập)"}, http.StatusUnauthorized, nil
	}

	// 3. Bind dữ liệu Request từ Body
	var input models.VirtualTryOnRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	// 4. Lấy provider mặc định từ Settings
	providerName := strings.ToLower(utils.GetSetting(c, "try_on_default_provider", "tutaoweb"))

	// 5. Khởi tạo Provider từ Factory
	provider, err := GetTryOnProvider(providerName)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusBadRequest, nil
	}

	// 6. Gọi API Provider
	result, execErr := provider.Execute(c, input)

	// 7. TỰ ĐỘNG LƯU LOG VÀO BẢNG try_on_requests (TẬP TRUNG TẠI ĐÂY, KHÔNG BỊ LẶP)
	var statusDB int = 1
	var resultURL string = ""
	var rawData interface{}

	if result != nil {
		resultURL = result.ResultURL
		rawData = result.RawData
	}

	if execErr != nil || resultURL == "" {
		statusDB = 0 // 0: Thất bại
		if rawData == nil {
			rawData = gin.H{"error": execErr.Error()}
		}
	}

	// Gọi hàm ghi log DB duy nhất
	_ = saveTryOnLog(c.Request.Context(), db, customerID, input.ProductID, providerName, statusDB, resultURL, rawData)

	// 8. Trả kết quả về cho Client
	if execErr != nil {
		return gin.H{"error": "Thử đồ thất bại: " + execErr.Error(), "data": rawData, "success": false}, http.StatusInternalServerError, execErr
	}

	return gin.H{
		"message":  "Xử lý thử đồ thành công",
		"provider": providerName,
		"data":     result,
		"success":  true,
	}, http.StatusOK, nil
}

// Helper: Tự động parse customerID từ c.Get("customerID") với nhiều kiểu dữ liệu (int, int64, float64, string)
func getCustomerIDFromContext(c *gin.Context) int64 {
	val, exists := c.Get("customerID")
	utils.LogToFile(val)
	utils.LogToFile(exists)

	utils.LogToFile(val)
	if !exists {
		return 0
	}

	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		id, _ := strconv.ParseInt(v, 10, 64)
		return id
	default:
		return 0
	}
}

// Helper: Lưu kết quả duy nhất vào CSDL
func saveTryOnLog(ctx context.Context, db *sqlx.DB, customerID int64, productID int64, providerID string, status int, resultURL string, apiResponse interface{}) error {
	jsonBytes, _ := json.Marshal(apiResponse)
	apiResponseStr := string(jsonBytes)

	var resultImage interface{}
	if resultURL != "" {
		resultImage = resultURL
	} else {
		resultImage = nil
	}

	query := `
		INSERT INTO try_on_requests 
		(product_id, customer_id, provider_id, result_image, status, api_response, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())`

	_, err := db.ExecContext(ctx, query,
		productID,
		customerID,
		providerID,
		resultImage,
		status,
		apiResponseStr,
	)

	return err
}
