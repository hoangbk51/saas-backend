package models

type CreateOrderHistoryRequest struct {
	OrderID int64  `json:"order_id"` // 👈 Đổi kiểu string -> int64
	Note    string `json:"note"`
	Status  int64  `json:"status"` // Tương ứng order_status_id (1: Confirmed, 2: Waiting for payment, v.v.)
}
