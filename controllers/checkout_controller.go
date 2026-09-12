package controllers

import (
	"context"
	"fmt"
	"go-saas/models"
	"go-saas/services"
	_ "go-saas/services/payment_gateway"
	"go-saas/tasks"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func CheckoutHandler(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "Không thể kết nối DB"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "Không thể tạo Transaction"})
		return
	}

	// Biến đánh dấu xem Transaction đã hoàn tất (Commit/Rollback) chưa
	txCommitted := false

	defer func() {
		// Nếu có Panic hoặc chưa Commit thành công thì Rollback an toàn
		if r := recover(); r != nil {
			if !txCommitted {
				tx.Rollback()
			}
			utils.LogToFile("PANIC trong CheckoutHandler: %v", r)
			c.JSON(500, gin.H{"success": false, "message": fmt.Sprintf("Server Panic: %v", r)})
		} else if !txCommitted {
			// Nếu handler kết thúc giữa chừng do return error mà quên Rollback
			tx.Rollback()
		}
	}()

	// 1. Bind JSON Request
	var req models.OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogToFile("Lỗi Validate OrderRequest: %v", err)
		c.JSON(400, gin.H{"success": false, "message": "Dữ liệu không hợp lệ", "error": err.Error()})
		return
	}

	// 2. Lấy Cart
	cartID := c.Param("id")
	cart, err := services.GetCartById(c, utils.StringToInt64(cartID))
	if err != nil {
		utils.LogToFile("Lỗi GetCartById: %v", err)
		c.JSON(404, gin.H{"success": false, "message": "Không tìm thấy giỏ hàng"})
		return
	}

	// 3. Tính toán lại Cart
	cartData, err := services.CartRecalculate(c, cart)
	if err != nil {
		utils.LogToFile("Lỗi CartRecalculate: %v", err)
		c.JSON(500, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 4. Lưu Order vào DB (chưa Commit)
	order, err := services.SaveOrderFromCart(c, tx, req, cartData)
	if err != nil {
		utils.LogToFile("Lỗi SaveOrderFromCart: %v", err) // 🎯 Xem log này nếu bị lỗi insert items/totals!
		c.JSON(500, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 5. Tạo Transaction record
	transaction, err := services.CreateTransaction(tx, order)
	if err != nil {
		utils.LogToFile("Lỗi CreateTransaction: %v", err)
		c.JSON(500, gin.H{"success": false, "message": "Không thể tạo bản ghi giao dịch"})
		return
	}

	// 6. CHỐT DỮ LIỆU ĐƠN HÀNG VÀO DATABASE DỰ ÁN
	if err := tx.Commit(); err != nil {
		utils.LogToFile("Lỗi tx.Commit(): %v", err)
		c.JSON(500, gin.H{"success": false, "message": "Lỗi khi chốt đơn hàng"})
		return
	}
	// Đánh dấu Transaction đã Commit xong! Từ đây về sau KHÔNG gọi tx.Commit() hay tx.Rollback() nữa!
	txCommitted = true

	// 📩 GỬI MAIL ĐƠN HÀNG BẤT ĐỒNG BỘ

	reps := map[string]any{
		"CustomerName": order.BillingFirstName + " " + order.BillingLastName,
		"OrderNumber":  order.OrderNumber,
		"GrandTotal":   order.GrandTotal,
		"Items":        cartData.Items,   // Danh sách sản phẩm từ giỏ hàng
		"Totals":       cartData.Charges, // Danh sách phụ phí / tổng tiền
	}
	_ = tasks.EnqueueDynamicEmail(utils.AsynqClient, c.GetString("tenantId"), order.Email, "order_created", reps)

	message := "🔔 <b>[Thông báo từ Web]</b>\nCó một đơn hàng mới vừa được tạo thành công!"

	fmt.Println("Đang gửi tin nhắn...")
	err = services.SendTelegramMessage(message)
	if err != nil {
		fmt.Printf("❌ Gửi tin nhắn thất bại: %v\n", err)
		return
	}
	fmt.Println("✅ Gửi tin nhắn thành công!")

	orderData := map[string]interface{}{
		"order_id":      order.ID,
		"customer_name": order.BillingFirstName,
		"total_amount":  order.GrandTotal,
	}
	var notifiableID uint64 = 0
	notify := models.Notification{
		Type:           "NEW_ORDER",
		NotifiableType: "App\\Models\\Tenant\\User",
		NotifiableID:   &notifiableID, // Để nil vì thông báo này gửi cho cả nhóm Admin

		// Gán role hoặc danh sách permission nhận thông báo này:
		//TargetRole:        utils.StringPtr("ORDER_MANAGER"),
		TargetPermissions: []string{"notify_order"}, // (Chọn 1 trong 2 hoặc truyền cả 2)

		Icon:       utils.StringPtr("shopping-cart"),
		ActionText: utils.StringPtr("Xem đơn hàng"),
		ActionURL:  utils.StringPtr(fmt.Sprintf("/admin/orders/%d", order.ID)),
		Message:    utils.StringPtr(fmt.Sprintf("Có đơn hàng mới #%d", order.ID)),
		Data:       orderData,
	}

	// Gọi Goroutine bất đồng bộ
	tenantID := c.GetString("tenantId")

	go func(tID string, tDB *sqlx.DB) {
		err := services.CreateAndBroadcastToTargetUsers(context.Background(), tDB, tID, notify)
		if err != nil {
			utils.LogToFile("[Notification Error] %v", err)
		}
	}(tenantID, db)

	// 7. XỬ LÝ THANH TOÁN (Sau khi đơn hàng đã chốt vào DB)
	paymentMethod, _ := services.GetPaymentMethodByCode(c, order.PaymentMethodCode)

	if paymentMethod.Code == "cod" {
		c.JSON(200, gin.H{
			"success":  true,
			"message":  "Đặt hàng thành công (COD)",
			"order_id": order.ID,
		})
		return
	}

	// Thanh toán qua cổng Online (VNPay, PayOS,...)
	provider, exists := services.GetPaymentGateway(paymentMethod.Code)
	if !exists {
		c.JSON(400, gin.H{"success": false, "message": "Phương thức thanh toán không hỗ trợ"})
		return
	}

	result, err := provider.Pay(c, order, transaction)
	if err != nil {
		utils.LogToFile("Lỗi Provider Pay: %v", err)
		c.JSON(500, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
func ShippingMethodChange(c *gin.Context) {
	// 1. Lấy dữ liệu từ Request Payload
	var input struct {
		ShippingMethodCode string `json:"shipping_method_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	// 2. Lấy IP khách (dùng để tìm currentCart)
	clientIP := c.ClientIP()

	// 3. Gọi Service xử lý
	result, err := services.UpdateShippingMethod(c, input.ShippingMethodCode, clientIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// 4. Trả về kết quả thành công (giống hệt Laravel Response)
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Cập nhật phương thức vận chuyển thành công",
		"cart":    result,
	})
}
