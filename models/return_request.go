package models

import "time"

type ReturnItemInput struct {
	OrderItemID int64  `json:"order_item_id" binding:"required"`
	Quantity    int    `json:"quantity" binding:"required,gt=0"`
	Reason      string `json:"reason"`
}

type CreateReturnRequest struct {
	OrderID int64             `json:"order_id" binding:"required"`
	Type    string            `json:"type" binding:"required,oneof=return exchange"`
	Reason  string            `json:"reason" binding:"required"`
	Note    string            `json:"note"`
	Items   []ReturnItemInput `json:"items" binding:"required,dive"`
}

type ReturnRequest struct {
	ID         int64     `json:"id" db:"id"`
	Code       string    `json:"code" db:"code"`
	OrderID    int64     `json:"order_id" db:"order_id"`
	CustomerID int64     `json:"customer_id" db:"customer_id"`
	Type       string    `json:"type" db:"type"`
	Reason     string    `json:"reason" db:"reason"`
	Note       string    `json:"note" db:"note"`
	Status     string    `json:"status" db:"status"`
	AdminNote  *string   `json:"admin_note,omitempty" db:"admin_note"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type ReturnItem struct {
	ID              int64  `json:"id" db:"id"`
	ReturnRequestID int64  `json:"return_request_id" db:"return_request_id"`
	OrderItemID     int64  `json:"order_item_id" db:"order_item_id"`
	Quantity        int    `json:"quantity" db:"quantity"`
	Reason          string `json:"reason" db:"reason"`
}

type ReturnDetailResponse struct {
	ReturnRequest ReturnRequest `json:"return_request"`
	Items         []ReturnItem  `json:"items"`
	ProofImages   []string      `json:"proof_images"`
}

type CreateReturnRequestReq struct {
	OrderID    uint32  `json:"order_id" binding:"required"`
	CustomerID uint32  `json:"customer_id" `
	Reason     string  `json:"reason" binding:"required"`
	Note       *string `json:"note"`
}

type UpdateReturnRequestReq struct {
	Status    string  `json:"status" binding:"required"` // PENDING, APPROVED, REJECTED, COMPLETED
	AdminNote *string `json:"admin_note"`
}

type AcceptReturnRequestReq struct {
	AdminNote string `json:"admin_note"` // Ghi chú của Admin (Ví dụ: "Đã chấp nhận, vui lòng gửi hàng về địa chỉ...")
}
