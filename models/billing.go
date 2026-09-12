package models

import "time"

type BillingHistory struct {
	ID             uint64     `json:"id"`
	ClientID       uint64     `json:"client_id"`
	TenantID       string     `json:"tenant_id"`
	TotalAmount    *uint32    `json:"total_amount"`
	Status         uint8      `json:"status"`
	OrderEmail     *string    `json:"order_email"`
	OrderPhone     *string    `json:"order_phone"`
	PaymentMethod  *string    `json:"payment_method_code"` // Code gốc ở bảng orders
	PaymentName    *string    `json:"payment_name"`        // Tên lấy từ bảng payment_methods
	CouponCode     *string    `json:"coupon_code"`
	SubTotal       *uint32    `json:"sub_total"`
	Total          *uint32    `json:"total"`
	DiscountAmount *int32     `json:"discount_amount"`
	Note           *string    `json:"note"`
	CreatedAt      *time.Time `json:"created_at"`
	Type           int        `json:"type"` // 1: Gói web, 2: Mua plugin
	ObjectID       int        `json:"object_id"`
	ObjectName     string     `json:"object_name"` // Tên "Gói Web" hoặc Tên Plugin từ bảng modules
}

// Struct để map dữ liệu filter từ Query Params
type BillingFilter struct {
	Type      string `form:"type"`
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
}
