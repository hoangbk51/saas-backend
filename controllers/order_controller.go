package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetOrderDetail(c *gin.Context) {
	idStr := c.Param("id")
	OrderId, _ := strconv.Atoi(idStr)
	// Gọi tầng dịch vụ để bốc tách dữ liệu đơn hàng
	orderDetail, err := services.GetOrderDetail(c, OrderId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// Trả kết quả thành công rực rỡ
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   orderDetail,
	})
}

func UpdateOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID đơn hàng không hợp lệ"})
		return
	}

	var req models.UpdateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Dữ liệu payload không hợp lệ: " + err.Error()})
		return
	}

	// Map items
	var cartItems []models.CartItem
	for _, item := range req.Items {
		price := item.UnitPrice
		if price == 0 {
			price = item.Price
		}
		desc := item.ItemDescription
		if desc == "" {
			desc = item.Name
		}

		cartItems = append(cartItems, models.CartItem{
			ProductID:       item.ID,
			ItemDescription: desc,
			UnitPrice:       price,
			Quantity:        item.Quantity,
		})
	}

	// Lấy giá trị Billing từ payload
	billingFirstName := req.BillingFirstName
	billingAddress1 := req.BillingAddress1

	// Lấy giá trị Shipping từ payload
	shippingFirstName := req.ShippingFirstName
	shippingAddress1 := req.ShippingAddress1

	// Trường hợp client chỉ gửi Billing (Shipping giống Billing)
	if shippingFirstName == "" {
		shippingFirstName = billingFirstName
	}
	if shippingAddress1 == "" {
		shippingAddress1 = billingAddress1
	}

	paymentMethod := req.PaymentMethodCode
	if paymentMethod == "" {
		paymentMethod = req.PaymentMethod
	}

	ShippingMethodCode := req.ShippingMethodCode
	if ShippingMethodCode == "" {
		ShippingMethodCode = req.ShippingMethodCode
	}

	dto := models.UpdateOrderDTO{
		OrderID:            orderID,
		Email:              req.Email,
		CustomerPhone:      req.CustomerPhone,
		PaymentMethod:      paymentMethod,
		ShippingMethodCode: ShippingMethodCode,

		Status: req.Status,
		Note:   req.Note,

		// Gán chính xác từng phần
		BillingFirstName: req.BillingFirstName,
		BillingLastName:  req.BillingLastName,
		BillingAddress1:  req.BillingAddress1,
		BillingCity:      req.BillingCity,
		BillingState:     req.BillingState,
		BillingZip:       req.BillingZip,
		BillingCountry:   req.BillingCountry,

		ShippingFirstName: shippingFirstName,
		ShippingLastName:  req.ShippingLastName,
		ShippingAddress1:  shippingAddress1,
		ShippingCity:      req.ShippingCity,
		ShippingState:     req.ShippingState,
		ShippingZip:       req.ShippingZip,
		ShippingCountry:   req.ShippingCountry,

		Items: cartItems,
	}

	updatedOrder, err := services.UpdateOrderDetail(c, dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Cập nhật đơn hàng thành công",
		"data":    updatedOrder,
	})
}

func SearchOrders(c *gin.Context) {
	responseData, statusCode, _ := services.SearchOrdersService(c)
	c.JSON(statusCode, responseData)
}

func AddOrderHistory(c *gin.Context) {
	orderID := c.Param("id")

	var req models.CreateOrderHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Payload không hợp lệ: " + err.Error(),
		})
		return
	}

	responseData, statusCode, _ := services.AddOrderHistoryService(c, orderID, req)
	c.JSON(statusCode, responseData)
}
