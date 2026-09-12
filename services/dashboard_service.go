package services

import (
	"fmt"
	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func GetDashboardSummary(c *gin.Context) (*models.DashboardSummary, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối CSDL từ context: %v", err)
	}

	summary := &models.DashboardSummary{
		RecentOrders:      make([]map[string]interface{}, 0),
		TopViewedProducts: make([]map[string]interface{}, 0),
	}

	// -------------------------------------------------------------
	// 1. TỔNG SỐ: Sản phẩm, Đơn hàng, Tổng doanh thu, Customer
	// -------------------------------------------------------------
	_ = db.QueryRowContext(c, `SELECT COUNT(*) FROM products`).Scan(&summary.Totals.TotalProducts)
	_ = db.QueryRowContext(c, `SELECT COUNT(*) FROM orders`).Scan(&summary.Totals.TotalOrders)
	_ = db.QueryRowContext(c, `SELECT IFNULL(SUM(grand_total), 0) FROM orders`).Scan(&summary.Totals.TotalRevenue)
	_ = db.QueryRowContext(c, `SELECT COUNT(*) FROM customers`).Scan(&summary.Totals.TotalCustomers)

	// -------------------------------------------------------------
	// 2. CÁC ĐƠN HÀNG GẦN ĐÂY (Lấy 5-10 đơn mới nhất)
	// -------------------------------------------------------------
	recentOrdersQuery := `
		SELECT id, customer_id, grand_total, order_status_id, created_at 
		FROM orders 
		ORDER BY created_at DESC 
		LIMIT 10`

	rowsOrders, err := db.QueryContext(c, recentOrdersQuery)
	if err == nil {
		defer rowsOrders.Close()
		for rowsOrders.Next() {
			var id int64
			var customerID interface{}
			var grandTotal float64
			var status int64
			var createdAt string

			if err := rowsOrders.Scan(&id, &customerID, &grandTotal, &status, &createdAt); err == nil {
				summary.RecentOrders = append(summary.RecentOrders, map[string]interface{}{
					"id":          id,
					"customer_id": customerID,
					"grand_total": grandTotal,
					"status":      status,
					"created_at":  createdAt,
				})
			}
		}
	}

	// -------------------------------------------------------------
	// 3. TỔNG SỐ ĐƠN HÀNG VÀ DOANH THU 7 NGÀY & 30 NGÀY GẦN ĐÂY
	// -------------------------------------------------------------
	// 7 ngày gần đây
	chart7DaysQuery := `
		SELECT COUNT(*), IFNULL(SUM(grand_total), 0) 
		FROM orders 
		WHERE created_at >= NOW() - INTERVAL 7 DAY`
	_ = db.QueryRowContext(c, chart7DaysQuery).Scan(&summary.Chart7Days.TotalOrders, &summary.Chart7Days.TotalRevenue)

	// 30 ngày (1 tháng) gần đây
	chart30DaysQuery := `
		SELECT COUNT(*), IFNULL(SUM(grand_total), 0) 
		FROM orders 
		WHERE created_at >= NOW() - INTERVAL 30 DAY`
	_ = db.QueryRowContext(c, chart30DaysQuery).Scan(&summary.Chart30Days.TotalOrders, &summary.Chart30Days.TotalRevenue)

	// -------------------------------------------------------------
	// 4. SẢN PHẨM NHIỀU VIEW NHẤT (Top 10 theo cột view)
	// -------------------------------------------------------------
	topViewedQuery := `
    SELECT 
        id, 
        title, 
        IFNULL(view, 0) AS view
    FROM products 
    ORDER BY view DESC 
    LIMIT 10`

	rowsTopView, err := db.QueryContext(c, topViewedQuery)
	if err == nil {
		defer rowsTopView.Close()
		for rowsTopView.Next() {
			var id int64
			var rawTitle string
			var view int64

			if err := rowsTopView.Scan(&id, &rawTitle, &view); err == nil {
				summary.TopViewedProducts = append(summary.TopViewedProducts, map[string]interface{}{
					"id":    id,
					"title": utils.ParseFlexibleField(rawTitle), // Tái sử dụng util có sẵn
					"view":  view,
				})
			}
		}
	}

	return summary, nil
}
