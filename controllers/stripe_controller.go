package controllers

import (
	"encoding/json"
	"go-saas/services"
	"go-saas/utils"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/client"
	"github.com/stripe/stripe-go/v76/webhook"
)

func StripeHandleSuccess(c *gin.Context) {
	sessionID := c.Query("session_id")
	// Giả sử bạn truyền tenant_id qua URL hoặc lấy từ session/context

	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu thông tin xác thực"})
		return
	}

	// 1. Lấy Secret Key của Tenant từ Database

	// 2. Khởi tạo Stripe Client riêng cho Tenant này
	sc := &client.API{}
	sc.Init("", nil) //todo

	// 3. Sử dụng client 'sc' để lấy thông tin session
	// Lưu ý: Phải dùng sc.CheckoutSessions chứ không dùng session.Get
	s, err := sc.CheckoutSessions.Get(sessionID, nil)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy session trên Stripe của Tenant"})
		return
	}

	// 4. Kiểm tra và xử lý logic nghiệp vụ
	if s.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
		orderID := s.Metadata["order_id"]
		refCode := s.Metadata["reference_code"]
		utils.LogSQL(orderID)
		if orderID != "" {
			utils.LogSQL(refCode)
			// Cập nhật Database của bạn
			err = services.CheckoutCallBack(c, refCode)

			if err != nil {
				c.JSON(500, gin.H{"error": "Update transaction failed"})
				return
			}

		}

		c.JSON(http.StatusOK, gin.H{"status": "success"})

	} else {
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
	}
}

func StripeWebhookHandler(c *gin.Context) {
	const MaxBodyBytes = int64(65536)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Read body failed"})
		return
	}

	// 1. Xác thực chữ ký Stripe (Quan trọng để bảo mật)
	endpointSecret := "" //os.Getenv("STRIPE_WEBHOOK_SECRET") //todo
	sigHeader := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, sigHeader, endpointSecret)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature"})
		return
	}

	// 2. Xử lý sự kiện "checkout.session.completed"
	if event.Type == "checkout.session.completed" {
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Parsing session failed"})
			return
		}

		// Lấy reference_code hoặc order_id từ Metadata bạn đã gửi lên khi tạo session
		orderIDStr := session.Metadata["order_id"]
		refCode := session.Metadata["refence_code"]

		if orderIDStr != "" {
			utils.LogSQL(refCode)
			// Cập nhật Database của bạn
			err = services.CheckoutCallBack(c, refCode)

			if err != nil {
				c.JSON(500, gin.H{"error": "Update transaction failed"})
				return
			}

		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
