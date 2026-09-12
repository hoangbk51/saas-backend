package controllers

import (
	"go-saas/services"
	"go-saas/services/payment_gateway"
	"go-saas/utils"
	"log"

	"github.com/gin-gonic/gin"
)

func PayPalCallbackHandler(c *gin.Context) {
	// PayPal trả về ?token=ORDER_ID trong URL
	paypalOrderID := c.Query("token")
	if paypalOrderID == "" {
		c.JSON(400, gin.H{"error": "Token not found"})
		return
	}

	// Khởi tạo Provider (Nên lấy ClientID/Secret từ DB tùy theo tenant)
	provider := &payment_gateway.PayPalProvider{
		ClientID:     "AfZxSTWd0odjNSlXGUEizt9-NFrzSILX_LmXzPMWUS7uAxkSG91oOpY5oNuvnJMVGUfgqZjNJcOt9kPb",
		ClientSecret: "EOHTvu5jv4hNwws1whRjHfGrw6yVkWU0T24dcOfgC4rRp9NjBLnXbgvgNNhubMHxUibgzfyL25UU6CJU",
		BaseURL:      "https://api-m.sandbox.paypal.com",
	}

	// 1. Thực hiện Capture tiền
	capture, err := provider.CapturePayment(paypalOrderID)
	if err != nil {
		log.Printf("PayPal Capture Error: %v", err)
		c.JSON(500, gin.H{"error": "Payment capture failed"})
		return
	}

	// 2. Kiểm tra trạng thái và cập nhật DB
	if capture.Status == "COMPLETED" {
		// Lấy lại reference_id mà bạn đã gửi đi lúc Pay (tx.Code)
		referenceID := ""
		if len(capture.PurchaseUnits) > 0 {
			referenceID = capture.PurchaseUnits[0].ReferenceID
		}
		utils.LogSQL(referenceID)
		// Cập nhật Database của bạn
		err = services.CheckoutCallBack(c, referenceID)

		if err != nil {
			c.JSON(500, gin.H{"error": "Update transaction failed"})
			return
		}

		// Redirect về trang cám ơn hoặc thông báo thành công
		//c.Redirect(http.StatusFound, "http://hoangk53.central.test/checkout/success")
		return
	}

	c.JSON(400, gin.H{"status": "failed", "paypal_status": capture.Status})
}
