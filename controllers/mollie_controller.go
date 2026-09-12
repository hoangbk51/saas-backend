package controllers

import (
	"encoding/json"
	"go-saas/services"
	"go-saas/utils"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type MollieResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"` // các trạng thái: open, canceled, pending, authorized, expired, failed, paid
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Metadata map[string]interface{} `json:"metadata"`
}

func MollieCallback(c *gin.Context) {
	// 1. Lấy mã giao dịch từ URL (tương đương $code trong PHP)
	code := c.Query("code")
	utils.LogSQL("MollieCallback")
	utils.LogSQL(code)
	// 2. Truy vấn Database để lấy giao dịch và API Key của Tenant
	// transaction := db.Where("code = ?", code).First()
	apiKey := "test_CaruQFMaTUd23p3mbaFunS4NkEuQwq"

	transaction, _ := services.GetTransactionByCode(c, code)

	// Giả sử bạn lưu reference_id (ID của Mollie) trong database
	referenceID := transaction.ReferenceID

	// 3. Gọi API Mollie để kiểm tra trạng thái
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.mollie.com/v2/payments/"+referenceID, nil)

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể kết nối Mollie"})
		return
	}
	defer resp.Body.Close()

	var mollieResp MollieResponse
	if err := json.NewDecoder(resp.Body).Decode(&mollieResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi giải mã JSON"})
		return
	}

	// 4. Kiểm tra trạng thái "paid"
	if mollieResp.Status == "paid" {
		// logic: transaction.success()
		// logic: checkout_done(transaction.order_id)

		log.Printf("Mollie: Giao dịch %s đã thanh toán thành công", code)

		err := services.CheckoutCallBack(c, code) // Sử dụng hàm service của bạn
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Update database failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Success"})
	} else {
		// logic: transaction.status = 2 (failed/canceled)
		log.Printf("Mollie: Giao dịch %s thất bại với trạng thái: %s", code, mollieResp.Status)
		c.JSON(http.StatusOK, gin.H{"message": "Payment not paid", "status": mollieResp.Status})
	}
}
