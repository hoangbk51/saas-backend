package dto

type CreateShipmentRequest struct {
	Note string `json:"note"` // Ghi chú cho shipper (VD: Cho xem hàng, Gọi trước khi giao)
}

type CreateShipmentResponse struct {
	TrackingCode string  `json:"tracking_code"`
	OrderCode    string  `json:"order_code"`
	Fee          float64 `json:"fee"`
	Status       string  `json:"status"`
}
