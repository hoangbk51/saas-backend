package models

import "time"

// Struct chứa Filter query string gửi lên từ Client
type CustomerOrderReportFilter struct {
	Name       string `form:"name"`
	Email      string `form:"email"`
	StartDate  string `form:"start_date"`  // Định dạng YYYY-MM-DD
	EndDate    string `form:"end_date"`    // Định dạng YYYY-MM-DD
	SortName   string `form:"sort_name"`   // Mặc định: created_at (start_date/end_date)
	SortDirect string `form:"sort_direct"` // Mặc định: asc hoặc desc
	Page       int    `form:"page"`
	Limit      int    `form:"limit"`
}

// Item từng khách hàng trong báo cáo
type CustomerOrderReportItem struct {
	CustomerID       *int64     `json:"customer_id"`
	BillingFirstName *string    `json:"billing_first_name"`
	BillingLastName  *string    `json:"billing_last_name"`
	Email            *string    `json:"email"`
	StartDate        *time.Time `json:"start_date"`
	EndDate          *time.Time `json:"end_date"`
	TotalOrders      int64      `json:"total_orders"`
	TotalProducts    int64      `json:"total_products"`
	Total            float64    `json:"total"`
}
