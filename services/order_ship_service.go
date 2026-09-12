package services

import (
	"fmt"
	"go-saas/dto"
	"go-saas/shipping"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func SendOrderToCarrier(c *gin.Context, orderID int, req dto.CreateShipmentRequest) (*dto.CreateShipmentResponse, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	// 1. Lấy Order
	order, err := GetOrderDetail(c, orderID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy đơn hàng: %v", err)
	}

	orderModel := utils.MapOrderDetailToOrder(order)

	// Kiểm tra nếu đơn hàng đã được đẩy ship trước đó
	if order.TrackingCode != nil && *order.TrackingCode != "" {
		return nil, fmt.Errorf("đơn hàng đã được tạo mã vận đơn: %s", *order.TrackingCode)
	}

	if order.ShippingMethodCode == "" {
		return nil, fmt.Errorf("đơn hàng chưa chọn phương thức vận chuyển")
	}

	// 2. Lấy Provider tương ứng (viettelpost, ghn, ghtk...)
	provider, ok := shipping.Get(order.ShippingMethodCode)
	if !ok {
		return nil, fmt.Errorf("hãng vận chuyển %s chưa được tích hợp", order.ShippingMethodCode)
	}

	// 3. Gọi Provider tạo đơn ship
	trackingCode, actualFee, err := provider.CreateOrder(c, orderModel, req.Note)
	if err != nil {
		return nil, fmt.Errorf("lỗi tạo vận đơn: %v", err)
	}

	// 4. Cập nhật Tracking Code & trạng thái Đơn hàng trong DB
	//statusProcessing := 11
	query := `
		UPDATE orders 
		SET tracking_id = ?, 
			
			order_status_id = ?, 
			updated_at = NOW() 
		WHERE id = ?`
	//shipping_fee_actual = ?,
	_, err = db.ExecContext(c.Request.Context(), query, trackingCode, 5, orderID)
	if err != nil {
		return nil, fmt.Errorf("lỗi lưu vận đơn vào database: %v", err)
	}

	return &dto.CreateShipmentResponse{
		TrackingCode: trackingCode,
		OrderCode:    fmt.Sprintf("ORD-%d", order.ID),
		Fee:          actualFee,
		//Status:       statusProcessing,
	}, nil
}

func CancelShipment(c *gin.Context, orderID int, req dto.CreateShipmentRequest) (*dto.CreateShipmentResponse, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	// 1. Lấy Order
	order, err := GetOrderDetail(c, orderID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy đơn hàng: %v", err)
	}
	trackingCode := order.TrackingCode
	provider, ok := shipping.Get(order.ShippingMethodCode)
	if !ok {
		return nil, fmt.Errorf("hãng vận chuyển %s chưa được tích hợp", order.ShippingMethodCode)
	}

	// 3. Gọi Provider tạo đơn ship
	err = provider.CancelShipment(c, *trackingCode)
	if err != nil {
		return nil, fmt.Errorf("lỗi hủy vận đơn: %v", err)
	}

	// 4. Cập nhật Tracking Code & trạng thái Đơn hàng trong DB
	//statusProcessing := 11
	query := `
		UPDATE orders 
		SET tracking_id = ?, 
			
			order_status_id = ?, 
			updated_at = NOW() 
		WHERE id = ?`
	//shipping_fee_actual = ?,
	_, err = db.ExecContext(c.Request.Context(), query, trackingCode, 8, orderID)
	if err != nil {
		return nil, fmt.Errorf("lỗi lưu vận đơn vào database: %v", err)
	}

	return &dto.CreateShipmentResponse{}, nil
}
