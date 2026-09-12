package models

// 1. Struct cho Báo cáo 1: Group theo Sản phẩm (Mỗi sản phẩm 1 dòng, cộng gộp tổng kho)
type ProductStockGroupReport struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	TotalStock  int64  `json:"total_stock"`
}

// 2. Struct cho Báo cáo 2: Liệt kê chi tiết theo Variant (Mỗi biến thể 1 dòng, sản phẩm đơn lẻ 1 dòng)
type VariantStockDetailReport struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	VariantID   *int64  `json:"variant_id"`   // Dùng pointer để linh hoạt trả về null nếu sản phẩm không có biến thể
	VariantName *string `json:"variant_name"` // Tên/Thuộc tính biến thể (VD: "Size M / Đỏ")
	Stock       int64   `json:"stock"`
}
