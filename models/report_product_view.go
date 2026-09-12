package models

// Struct chứa các tham số Filter từ Query String
type ProductViewReportFilter struct {
	Title     string `form:"title"`
	ProductID string `form:"product_id"`
	Page      int    `form:"page"`
	Limit     int    `form:"limit"`
}
