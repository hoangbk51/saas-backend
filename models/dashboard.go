package models

// Struct tổng chứa toàn bộ dữ liệu Dashboard
type DashboardSummary struct {
	Totals            DashboardTotals          `json:"totals"`
	RecentOrders      []map[string]interface{} `json:"recent_orders"`
	Chart7Days        DashboardChartData       `json:"chart_7_days"`
	Chart30Days       DashboardChartData       `json:"chart_30_days"`
	TopViewedProducts []map[string]interface{} `json:"top_viewed_products"`
}

// 1. Thống kê tổng số
type DashboardTotals struct {
	TotalProducts  int64   `json:"total_products"`
	TotalOrders    int64   `json:"total_orders"`
	TotalRevenue   float64 `json:"total_revenue"`
	TotalCustomers int64   `json:"total_customers"`
}

// 3. Dữ liệu biểu đồ theo chuỗi thời gian
type DashboardChartData struct {
	TotalOrders  int64   `json:"total_orders"`
	TotalRevenue float64 `json:"total_revenue"`
}
