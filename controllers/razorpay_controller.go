package controllers

import (
	"encoding/json"
	"fmt"
	"go-saas/services"
	"go-saas/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func RazorpayCallback(c *gin.Context) {
	code := c.Param("code")
	// Lấy payment_id từ request gửi lên
	//paymentID := c.Query("razorpay_payment_id")
	paymentID := c.PostForm("razorpay_payment_id")
	utils.LogSQL("paymentID: " + paymentID)

	db, _ := utils.GetDBFromContext(c)
	key := "rzp_test_PEMGZ0g2W0kpeq"
	secret := "jSZffOIDHKys8LNgyfRHVxqE"

	// 1. GỌI ĐÚNG ENDPOINT (phải có ID ở cuối)
	apiURL := fmt.Sprintf("https://api.razorpay.com/v1/payments/%s", paymentID)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.SetBasicAuth(key, secret)

	resp, err := client.Do(req)
	if err != nil {
		utils.LogSQL("Razorpay Connection Error: " + err.Error())
		return
	}
	defer resp.Body.Close()

	// 2. DECODE VÀO MAP (Hoặc Struct)
	var paymentData map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&paymentData)

	// Log để kiểm tra (lần này sẽ không ra entity: collection nữa mà ra entity: payment)
	utils.LogSQL(fmt.Sprintf("Razorpay Single Payment Data: %v", paymentData))

	// 3. KIỂM TRA TRẠNG THÁI
	status, _ := paymentData["status"].(string)

	// Razorpay status: authorized (đã xác thực) hoặc captured (đã thu tiền)
	if status == "authorized" || status == "captured" {
		// Cập nhật Database
		query := `UPDATE transactions SET reference_id = ?, status = 1 WHERE code = ?`
		utils.LogSQL(query, paymentID, code)

		_, err := db.Exec(query, paymentID, code)

		if err != nil {
			utils.LogSQL("DB Update Error: " + err.Error())
			return
		}

		// Hoàn tất đơn hàng
		services.CheckoutCallBack(c, code)

		// Điều hướng khách hàng về trang thành công
		//c.Redirect(http.StatusSeeOther, "/checkout/success")
	} else {
		utils.LogSQL("Payment not successful. Status: " + status)
		c.JSON(http.StatusOK, gin.H{"message": "Payment failed", "status": status})
	}
}
