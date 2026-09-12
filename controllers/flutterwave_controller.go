package controllers

import (
	"encoding/json"
	"fmt"
	"go-saas/services"
	"go-saas/utils"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type FlutterwaveVerifyResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID       int64   `json:"id"`
		TxRef    string  `json:"tx_ref"`
		FlwRef   string  `json:"flw_ref"`
		Status   string  `json:"status"`
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"data"`
}

func FlutterwaveCallbackHandler(c *gin.Context) {
	// 1. Lấy thông tin từ Query Params mà Flutterwave gửi về
	// URL thường có dạng: ?status=successful&tx_ref=POPSHOP_123&transaction_id=456
	status := c.Query("status")
	transactionID := c.Query("transaction_id") // ID từ phía Flutterwave

	if status != "successful" {
		// Xử lý khi cancelled hoặc failed
		c.Redirect(http.StatusFound, "http://hoangk66.central.test/checkout/payment-failed")
		return
	}

	// 2. Gọi API Verify lại từ Server (Bắt buộc để bảo mật)
	verifiedData, err := verifyFlutterwaveTransaction(transactionID)
	if err != nil || verifiedData.Data.Status != "successful" {
		log.Printf("Verify thất bại: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Transaction verification failed"})
		return
	}

	// Tìm transaction theo tx_ref (chính là code bạn gửi đi lúc trước)
	txCode := verifiedData.Data.TxRef
	//flwRef := verifiedData.Data.FlwRef // Mã tham chiếu của Flutterwave

	utils.LogSQL(txCode)
	// Cập nhật Database của bạn
	err = services.CheckoutCallBack(c, txCode)

	if err != nil {
		c.JSON(500, gin.H{"error": "Update transaction failed"})
		return
	}

	// Redirect khách hàng về trang hoàn tất
	//c.Redirect(http.StatusFound, fmt.Sprintf("http://hoangk53.central.test/checkout/done/%d", orderID))
}

// Hàm hỗ trợ gọi API Verify của Flutterwave
func verifyFlutterwaveTransaction(transactionID string) (*FlutterwaveVerifyResponse, error) {
	secretKey := "" // Thay bằng Secret Key của bạn todo
	url := fmt.Sprintf("https://api.flutterwave.com/v3/transactions/%s/verify", transactionID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result FlutterwaveVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
