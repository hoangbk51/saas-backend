package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type AuthNetRequest struct {
	CreateTransactionRequest struct {
		MerchantAuthentication struct {
			Name           string `json:"name"`
			TransactionKey string `json:"transactionKey"`
		} `json:"merchantAuthentication"`
		RefId              string `json:"refId"`
		TransactionRequest struct {
			TransactionType string `json:"transactionType"`
			Amount          string `json:"amount"`
			Payment         struct {
				CreditCard struct {
					CardNumber     string `json:"cardNumber"`
					ExpirationDate string `json:"expirationDate"`
					CardCode       string `json:"cardCode"`
				} `json:"creditCard"`
			} `json:"payment"`
			// 1. Order PHẢI nằm trước Customer và BillTo
			Order struct {
				InvoiceNumber string `json:"invoiceNumber"`
			} `json:"order"`
			// 2. Customer nằm sau Order
			Customer struct {
				Id    string `json:"id"`
				Email string `json:"email"`
			} `json:"customer"`
			// 3. BillTo nằm sau Customer
			BillTo struct {
				LastName string `json:"lastName"`
				Address  string `json:"address"`
				City     *int64 `json:"city"`
				Zip      string `json:"zip"`
				Country  string `json:"country"`
			} `json:"billTo"`
		} `json:"transactionRequest"`
	} `json:"createTransactionRequest"`
}

func AuthorizeNetPaymentSubmit(c *gin.Context) {
	code := c.Param("code")
	db, _ := utils.GetDBFromContext(c)
	utils.LogSQL("code :" + code)
	// 1. Lấy thông tin Transaction & Order từ DB
	var transaction models.Transaction
	db.Get(&transaction, "SELECT * FROM transactions WHERE code = ?", code)
	utils.LogSQL("OrderID :?", transaction.OrderID)

	var order models.Order
	//db.Get(&order, "SELECT * FROM orders WHERE id = ?", transaction.OrderID)
	err := db.QueryRowx("SELECT grand_total FROM orders WHERE id = ?", transaction.OrderID).Scan(&order.GrandTotal)
	if err != nil {
		utils.LogSQL("Lỗi Scan GrandTotal: " + err.Error())
	}
	// 2. Lấy Config Authorize.net của Tenant

	// 3. Chuẩn bị Payload cho Authorize.net
	payload := AuthNetRequest{}
	payload.CreateTransactionRequest.MerchantAuthentication.Name = "9K62kCb8GmA"                //config.LoginID
	payload.CreateTransactionRequest.MerchantAuthentication.TransactionKey = "7M45LFS2uwq2z9RZ" // config.TransactionKey
	payload.CreateTransactionRequest.RefId = fmt.Sprintf("ref%d", time.Now().Unix())

	txRequest := &payload.CreateTransactionRequest.TransactionRequest
	txRequest.TransactionType = "authCaptureTransaction"
	fmt.Println("GrandTotal:", order.GrandTotal)
	txRequest.Amount = fmt.Sprintf("%.2f", order.GrandTotal)

	// Lấy data từ Form POST (từ view gửi qua)
	txRequest.Payment.CreditCard.CardNumber = strings.ReplaceAll(c.PostForm("cardNumber"), " ", "")
	txRequest.Payment.CreditCard.ExpirationDate = c.PostForm("expiration-year") + "-" + c.PostForm("expiration-month")
	txRequest.Payment.CreditCard.CardCode = c.PostForm("cvv")

	txRequest.Order.InvoiceNumber = fmt.Sprintf("%d%d", time.Now().Unix(), order.ID)
	txRequest.BillTo.LastName = order.ShippingLastName
	txRequest.BillTo.Address = order.ShippingAddress1
	txRequest.BillTo.City = order.ShippingCity
	txRequest.BillTo.Zip = order.ShippingZip
	txRequest.BillTo.Country = "US" // Hoặc lấy từ order.ShippingCountry.Name

	txRequest.Customer.Id = fmt.Sprintf("%d", order.CustomerID)
	txRequest.Customer.Email = order.Email

	// 4. Gọi API Authorize.net
	apiURL := "https://apitest.authorize.net/xml/v1/request.api" // Sandbox
	//if config.IsLive {  //todo
	//	apiURL = "https://api.authorize.net/xml/v1/request.api" // Production
	//}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		utils.LogSQL("AuthNet Connection Error: " + err.Error())
		c.JSON(500, gin.H{"message": "Payment system unavailable"})
		return
	}
	defer resp.Body.Close()

	// 5. Đọc và Parse kết quả
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyBytes = bytes.TrimPrefix(bodyBytes, []byte("\xef\xbb\xbf")) // Xóa BOM nếu có

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		utils.LogSQL("Lỗi Parse JSON: " + err.Error())
		c.JSON(500, gin.H{"message": "Lỗi xử lý phản hồi từ hệ thống"})
		return
	}

	// 6. Kiểm tra kết quả thanh toán
	messages, _ := result["messages"].(map[string]interface{})
	if messages["resultCode"] == "Ok" {
		txResp, _ := result["transactionResponse"].(map[string]interface{})
		transID, _ := txResp["transId"].(string)

		// Cập nhật trạng thái thành công
		db.Exec("UPDATE transactions SET reference_id = ?, status = 1 WHERE code = ?", transID, code)

		services.CheckoutCallBack(c, code)
		c.Redirect(http.StatusSeeOther, "/success")
	} else {
		utils.LogSQL("AuthNet Failed: " + string(bodyBytes))
		c.JSON(400, gin.H{"message": "Thanh toán bị từ chối, vui lòng kiểm tra lại thẻ"})
	}
}
