package controllers

import (
	"net/http"
	"strconv"

	"go-saas/dto"
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

// SendToCarrierHandler godoc
// @Summary Admin gửi đơn hàng sang đơn vị vận chuyển
// @Tags Admin Orders
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Param request body dto.CreateShipmentRequest true "Cấu hình gửi đơn"
// @Success 200 {object} dto.CreateShipmentResponse
// @Router /api/admin/orders/{id}/create-shipment [post]
func SendToCarrierHandler(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID đơn hàng không hợp lệ"})
		return
	}

	var req dto.CreateShipmentRequest
	_ = c.ShouldBindJSON(&req) // Note có thể empty

	res, err := services.SendOrderToCarrier(c, orderID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Đã tạo đơn thành công trên hệ thống nhà vận chuyển",
		"data":    res,
	})
}

func CancelShipment(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))

	var req dto.CreateShipmentRequest
	_ = c.ShouldBindJSON(&req) // Note có thể empty

	_, err = services.CancelShipment(c, orderID, req)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Hủy đơn vận chuyển GoShip thành công!",
	})
}
