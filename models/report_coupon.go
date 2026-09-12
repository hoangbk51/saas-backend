package models

// Struct chứa các tham số Filter từ Query String
type CouponReportFilter struct {
	CouponName string `form:"coupon_name"`
	CouponCode string `form:"coupon_code"`
	StartDate  string `form:"start_date"` // Định dạng YYYY-MM-DD
	EndDate    string `form:"end_date"`   // Định dạng YYYY-MM-DD
	Page       int    `form:"page"`
	Limit      int    `form:"limit"`
}
