package controllers

import (
	"encoding/json"
	"go-saas/services"
	"go-saas/utils"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

type SSLCommerzResponse struct {
	Status       string `json:"status"`
	Amount       string `json:"amount"` // SSL trả về "100.00" (string)
	Currency     string `json:"currency"`
	TranID       string `json:"tran_id"`
	ValID        string `json:"val_id"`
	StoreAmount  string `json:"store_amount"` // Số tiền sau khi trừ phí
	CardType     string `json:"card_type"`
	ErrorCode    string `json:"error_code"`    // Có thể trống nếu thành công
	ErrorMessage string `json:"error_message"` // Có thể trống nếu thành công
}

func SSLCommerzCallback(c *gin.Context) {
	// 1. SSLCommerz gửi data qua POST body (Form-data)
	tranID := c.PostForm("tran_id") // Đây là mã code của mình gửi đi lúc nãy
	valID := c.PostForm("val_id")   // ID xác thực từ SSL
	status := c.PostForm("status")

	utils.LogSQL("SSLCallback: Code=" + tranID + ", ValID=" + valID)

	if status != "VALID" && status != "AUTHENTICATED" {
		c.JSON(http.StatusOK, gin.H{"message": "Payment failed", "status": status})
		return
	}

	// 2. Lấy API Key của Tenant để gọi Validate (Bắt buộc để bảo mật)
	storeID := "tutao645310f10bf48"
	storePass := "tutao645310f10bf48@ssl"

	mode := "sandbox" // hoặc "live"

	// 1. Xác định Base URL chính xác
	var validationURL string
	if mode == "sandbox" {
		validationURL = "https://sandbox.sslcommerz.com/validator/api/merchantTransIDvalidationAPI.php"
	} else {
		validationURL = "https://securepay.sslcommerz.com/validator/api/merchantTransIDvalidationAPI.php"
	}

	// 2. Sử dụng url.Values để build Query String (tránh lỗi ký tự đặc biệt)
	params := url.Values{}
	params.Set("val_id", valID)
	params.Set("store_id", storeID)
	params.Set("store_passwd", storePass)
	params.Set("format", "json")

	finalURL := validationURL + "?" + params.Encode()

	// Log để kiểm tra URL cuối cùng có đuôi .php chưa
	utils.LogSQL("Final Validation URL: " + finalURL)

	// 3. Gọi Request
	resp, err := http.Get(finalURL)
	if err != nil {
		utils.LogSQL("HTTP Get Error: " + err.Error())
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	if bodyStr == "File not found." {
		utils.LogSQL("CRITICAL: SSLCommerz returned 404. Check your URL path!")
		c.JSON(400, gin.H{"error": "Invalid Validation Endpoint"})
		return
	}

	// 3. Giải mã và bắt lỗi
	var result SSLCommerzResponse
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		// Nếu dòng này in ra lỗi, bạn sẽ biết trường nào bị sai kiểu dữ liệu
		utils.LogSQL("JSON Decode Error: " + err.Error())
	}

	utils.LogSQL("Status sau khi parse: " + result.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Validation failed"})
		return
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&result)
	utils.LogSQL("SSLCallback: status=" + result.Status)

	// 4. Kiểm tra status từ Validation API
	if result.Status == "VALID" || result.Status == "VALIDATED" {
		// Update DB: dùng tranID (chính là code) để tìm
		db, _ := utils.GetDBFromContext(c)
		_, err = db.Exec("UPDATE transactions SET reference_id = ?, status = 1 WHERE code = ? ",
			valID, tranID)

		if err != nil {
			utils.LogSQL("Update DB failed: " + err.Error())
		}

		services.CheckoutCallBack(c, tranID) // Hoàn tất đơn hàng
		//c.Redirect(http.StatusSeeOther, "/payment-success")
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Fake payment detected"})
	}
}
