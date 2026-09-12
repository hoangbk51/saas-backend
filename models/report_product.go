package models

// Struct chứa các tham số Filter truyền lên từ Query String
type ProductSellReportFilter struct {
	Title     string `form:"title"`
	ProductID string `form:"product_id"`
	Page      int    `form:"page"`
	Limit     int    `form:"limit"`
}
