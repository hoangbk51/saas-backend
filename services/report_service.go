package services

import (
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// BÁO CÁO 1: Gom nhóm theo Sản phẩm (Group by Product)
func GetStockProduct(c *gin.Context) ([]models.ProductStockGroupReport, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT 
			p.id AS product_id,
			-- Trích xuất tiếng Việt từ JSON title, nếu null thì fallback lấy nguyên JSON hoặc chuỗi rỗng
			IFNULL(p.title->>'$.vi', IFNULL(p.title, '')) AS product_name,
			CASE 
				WHEN p.has_variant = 1 THEN IFNULL(SUM(pv.quantity), 0)
				ELSE IFNULL(p.stock_quantity, 0)
			END AS total_stock
		FROM products p
		LEFT JOIN product_variants pv ON p.id = pv.product_id
		GROUP BY p.id, p.title, p.has_variant, p.stock_quantity
		ORDER BY p.id DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn báo cáo tổng hợp: %v", err)
	}
	defer rows.Close()

	var reports []models.ProductStockGroupReport
	for rows.Next() {
		var item models.ProductStockGroupReport
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.TotalStock); err != nil {
			return nil, err
		}
		reports = append(reports, item)
	}

	return reports, nil
}

func GetStockVariant(c *gin.Context) ([]models.VariantStockDetailReport, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}
	query := `
		-- 1. Lấy các sản phẩm CÓ BIẾN THỂ (Gom tên option_values thành tên variant)
		SELECT 
			p.id AS product_id,
			IFNULL(p.title->>'$.vi', IFNULL(p.title, '')) AS product_name,
			pv.id AS variant_id,
			-- Gom các giá trị option thành chuỗi (VD: "M / Đỏ")
			IFNULL(
				GROUP_CONCAT(
					IFNULL(ov.name->>'$.vi', IFNULL(ov.name, '')) 
					ORDER BY o.id ASC 
					SEPARATOR ' / '
				), 
				CONCAT('Biến thể #', pv.id)
			) AS variant_name,
			IFNULL(pv.quantity, 0) AS stock
		FROM products p
		INNER JOIN product_variants pv ON p.id = pv.product_id
		-- JOIN qua bảng trung gian giữa variant và option_values (điều chỉnh tên bảng pvo nếu DB bạn đặt khác)
		LEFT JOIN product_variant_option_values pvo ON pv.id = pvo.variant_id
		LEFT JOIN option_values ov ON pvo.option_value_id = ov.id
		LEFT JOIN options o ON ov.option_id = o.id
		WHERE p.has_variant = 1
		GROUP BY p.id, p.title, pv.id, pv.quantity

		UNION ALL

		-- 2. Lấy các sản phẩm ĐƠN LẺ (Không có biến thể)
		SELECT 
			p.id AS product_id,
			IFNULL(p.title->>'$.vi', IFNULL(p.title, '')) AS product_name,
			NULL AS variant_id,
			NULL AS variant_name,
			IFNULL(p.stock_quantity, 0) AS stock
		FROM products p
		WHERE p.has_variant = 0 OR p.has_variant IS NULL

		ORDER BY product_id DESC, variant_id ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn báo cáo chi tiết: %v", err)
	}
	defer rows.Close()

	var reports []models.VariantStockDetailReport
	for rows.Next() {
		var item models.VariantStockDetailReport
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.VariantID, &item.VariantName, &item.Stock); err != nil {
			return nil, err
		}
		reports = append(reports, item)
	}

	return reports, nil
}

// Service nhận trực tiếp gin.Context/context.Context
func GetCustomerOrderReport(c *gin.Context, filter models.CustomerOrderReportFilter) (interface{}, error) {
	// 1. Lấy DB từ Context ngay trong Service
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối CSDL từ context: %v", err)
	}

	// 2. Set giá trị mặc định cho Pagination
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 15 // Mặc định như setting('elements_per_page')
	}

	// 3. Whitelist các cột ORDER BY để chống SQL Injection
	allowedSorts := map[string]string{
		"customer_id":        "customer_id",
		"billing_first_name": "billing_first_name",
		"email":              "email",
		"created_at":         "MIN(orders.created_at)",
		"start_date":         "MIN(orders.created_at)",
		"end_date":           "MAX(orders.created_at)",
		"total_orders":       "COUNT(*)",
		"total_products":     "SUM(orders.item_count)",
		"total":              "SUM(orders.grand_total)",
	}

	sortColumn, exists := allowedSorts[filter.SortName]
	if !exists {
		sortColumn = "MIN(orders.created_at)" // Mặc định
	}

	sortDirect := "ASC"
	if strings.ToLower(filter.SortDirect) == "desc" {
		sortDirect = "DESC"
	}

	// 4. Dựng điều kiện WHERE
	whereClauses := []string{"1=1"}
	args := []interface{}{}

	if filter.Name != "" {
		whereClauses = append(whereClauses, "billing_first_name LIKE ?")
		args = append(args, "%"+filter.Name+"%")
	}
	if filter.Email != "" {
		whereClauses = append(whereClauses, "email LIKE ?")
		args = append(args, "%"+filter.Email+"%")
	}
	if filter.StartDate != "" {
		whereClauses = append(whereClauses, "DATE(created_at) >= ?")
		args = append(args, filter.StartDate)
	}
	if filter.EndDate != "" {
		whereClauses = append(whereClauses, "DATE(created_at) <= ?")
		args = append(args, filter.EndDate)
	}

	whereStmt := strings.Join(whereClauses, " AND ")

	// 5. Đếm tổng số bản ghi (totalCount cho BuildLaravelPagination)
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT customer_id, billing_first_name, billing_last_name, email 
			FROM orders 
			WHERE %s 
			GROUP BY customer_id, billing_first_name, billing_last_name, email
		) AS total_groups`, whereStmt)

	var totalCount int64
	err = db.QueryRowContext(c, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("lỗi đếm tổng số trang: %v", err)
	}

	// 6. Query danh sách phân trang
	offset := (page - 1) * limit
	dataQuery := fmt.Sprintf(`
		SELECT 
			customer_id, 
			billing_first_name, 
			billing_last_name, 
			email,
			MIN(created_at) AS start_date,
			MAX(created_at) AS end_date,
			COUNT(*) AS total_orders,
			IFNULL(SUM(item_count), 0) AS total_products,
			IFNULL(SUM(grand_total), 0) AS total
		FROM orders
		WHERE %s
		GROUP BY customer_id, billing_first_name, billing_last_name, email
		ORDER BY %s %s
		LIMIT ? OFFSET ?`, whereStmt, sortColumn, sortDirect)

	dataArgs := append(args, limit, offset)

	rows, err := db.QueryContext(c, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn báo cáo đơn hàng: %v", err)
	}
	defer rows.Close()

	// 1. Khai báo mảng kiểu []map[string]interface{}
	orders := make([]map[string]interface{}, 0)

	for rows.Next() {
		var item models.CustomerOrderReportItem
		err := rows.Scan(
			&item.CustomerID,
			&item.BillingFirstName,
			&item.BillingLastName,
			&item.Email,
			&item.StartDate,
			&item.EndDate,
			&item.TotalOrders,
			&item.TotalProducts,
			&item.Total,
		)
		if err != nil {
			return nil, err
		}

		// 2. Map dữ liệu vào map[string]interface{} để khớp với BuildLaravelPagination
		orderMap := map[string]interface{}{
			"customer_id":        item.CustomerID,
			"billing_first_name": item.BillingFirstName,
			"billing_last_name":  item.BillingLastName,
			"email":              item.Email,
			"start_date":         item.StartDate,
			"end_date":           item.EndDate,
			"total_orders":       item.TotalOrders,
			"total_products":     item.TotalProducts,
			"total":              item.Total,
		}

		orders = append(orders, orderMap)
	}

	// 3. Ép kiểu totalCount từ int64 sang int cho đúng signature của BuildLaravelPagination
	response := utils.BuildLaravelPagination(c, orders, int(totalCount), page, limit)
	return response, nil
}

func GetProductsSellReport(c *gin.Context, filter models.ProductSellReportFilter) (interface{}, error) {
	// 1. Lấy DB từ Context ngay trong Service
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể lấy kết nối DB từ context: %v", err)
	}

	// 2. Xử lý giá trị mặc định cho Pagination
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 15 // Tương đương setting('elements_per_page')
	}

	// 3. Dựng câu điều kiện WHERE động
	whereClauses := []string{"1=1"}
	args := []interface{}{}

	if filter.Title != "" {
		whereClauses = append(whereClauses, "item_description LIKE ?")
		args = append(args, "%"+filter.Title+"%")
	}

	if filter.ProductID != "" {
		whereClauses = append(whereClauses, "product_id LIKE ?")
		args = append(args, "%"+filter.ProductID+"%")
	}

	whereStmt := strings.Join(whereClauses, " AND ")

	// 4. Đếm tổng số nhóm (Total Count) cho Pagination
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT product_id 
			FROM order_items 
			WHERE %s 
			GROUP BY product_id
		) AS total_groups`, whereStmt)

	var totalCount int64
	err = db.QueryRowContext(c, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("lỗi đếm tổng số bản ghi sản phẩm đã bán: %v", err)
	}

	// 5. Query lấy dữ liệu thực tế (Phân trang + GroupBy)
	offset := (page - 1) * limit
	dataQuery := fmt.Sprintf(`
		SELECT 
			product_id, 
			item_description, 
			IFNULL(SUM(quantity), 0) AS quantity, 
			IFNULL(SUM(unit_price * quantity), 0) AS amount
		FROM order_items
		WHERE %s
		GROUP BY product_id, item_description
		ORDER BY MAX(created_at) DESC
		LIMIT ? OFFSET ?`, whereStmt)

	dataArgs := append(args, limit, offset)

	rows, err := db.QueryContext(c, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn báo cáo sản phẩm đã bán: %v", err)
	}
	defer rows.Close()

	// 6. Map dữ liệu ra []map[string]interface{} để khớp với BuildLaravelPagination
	items := make([]map[string]interface{}, 0)

	for rows.Next() {
		var productID interface{}
		var itemDescription *string
		var quantity float64
		var amount float64

		err := rows.Scan(&productID, &itemDescription, &quantity, &amount)
		if err != nil {
			return nil, err
		}

		itemMap := map[string]interface{}{
			"product_id":       productID,
			"item_description": itemDescription,
			"quantity":         quantity,
			"amount":           amount,
		}

		items = append(items, itemMap)
	}

	// 7. Gọi helper build pagination chuẩn Laravel (ép kiểu totalCount về int)
	response := utils.BuildLaravelPagination(c, items, int(totalCount), page, limit)
	return response, nil
}

func GetProductsViewReport(c *gin.Context, filter models.ProductViewReportFilter) (interface{}, error) {
	// 1. Lấy DB Connection từ Context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể lấy kết nối DB từ context: %v", err)
	}

	// 2. Xử lý giá trị mặc định cho Pagination
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 15 // Mặc định như setting('elements_per_page')
	}

	// 3. Dựng câu điều kiện WHERE động
	whereClauses := []string{"1=1"}
	args := []interface{}{}

	if filter.Title != "" {
		// Tìm kiếm trong chuỗi JSON title
		whereClauses = append(whereClauses, "p.title LIKE ?")
		args = append(args, "%"+filter.Title+"%")
	}

	if filter.ProductID != "" {
		whereClauses = append(whereClauses, "p.id LIKE ?")
		args = append(args, "%"+filter.ProductID+"%")
	}

	whereStmt := strings.Join(whereClauses, " AND ")

	// 4. Đếm tổng số bản ghi (Total Count) cho Phân trang
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM products p 
		WHERE %s`, whereStmt)

	var totalCount int64
	err = db.QueryRowContext(c, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("lỗi đếm tổng số sản phẩm xem: %v", err)
	}

	// 5. Query lấy danh sách sản phẩm (Lượt xem)
	offset := (page - 1) * limit
	dataQuery := fmt.Sprintf(`
		SELECT 
			p.id, 
			IFNULL(p.title->>'$.vi', IFNULL(p.title, '')) AS title, 
			IFNULL(p.view, 0) AS view
		FROM products p
		WHERE %s
		ORDER BY p.created_at DESC
		LIMIT ? OFFSET ?`, whereStmt)

	dataArgs := append(args, limit, offset)

	rows, err := db.QueryContext(c, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn báo cáo lượt xem sản phẩm: %v", err)
	}
	defer rows.Close()

	// 6. Map dữ liệu sang []map[string]interface{}
	items := make([]map[string]interface{}, 0)

	for rows.Next() {
		var id int64
		var title string
		var view int64

		err := rows.Scan(&id, &title, &view)
		if err != nil {
			return nil, err
		}

		itemMap := map[string]interface{}{
			"id":    id,
			"title": title,
			"view":  view,
		}

		items = append(items, itemMap)
	}

	// 7. Gọi helper build pagination chuẩn Laravel (ép kiểu totalCount về int)
	response := utils.BuildLaravelPagination(c, items, int(totalCount), page, limit)
	return response, nil
}

func GetSalesReport(c *gin.Context, filter models.SalesReportFilter) (interface{}, error) {
	// 1. Lấy DB Connection từ Context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể lấy kết nối DB từ context: %v", err)
	}

	// 2. Xử lý giá trị mặc định cho Phân trang
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 15 // Mặc định như setting('elements_per_page')
	}

	// 3. Dựng điều kiện WHERE động theo ngày tạo đơn hàng
	whereClauses := []string{"1=1"}
	args := []interface{}{}

	if filter.StartDate != "" {
		whereClauses = append(whereClauses, "DATE(orders.created_at) >= ?")
		args = append(args, filter.StartDate)
	}

	if filter.EndDate != "" {
		whereClauses = append(whereClauses, "DATE(orders.created_at) <= ?")
		args = append(args, filter.EndDate)
	}

	whereStmt := strings.Join(whereClauses, " AND ")

	// 4. Đếm tổng số nhóm (Total Count) phục vụ Phân trang
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT orders.id 
			FROM orders 
			WHERE %s 
			GROUP BY orders.id
		) AS total_groups`, whereStmt)

	var totalCount int64
	err = db.QueryRowContext(c, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("lỗi đếm tổng số bản ghi báo cáo doanh số: %v", err)
	}

	// 5. Query lấy dữ liệu thống kê doanh số
	offset := (page - 1) * limit
	dataQuery := fmt.Sprintf(`
		SELECT 
			orders.id,
			MIN(orders.created_at) AS start_date,
			MAX(orders.created_at) AS end_date,
			COUNT(*) AS total_orders,
			IFNULL(SUM(orders.item_count), 0) AS total_products,
			IFNULL(SUM(orders.total), 0) AS sub_total,
			IFNULL(SUM(orders.shipping), 0) AS shipping_cost,
			IFNULL(SUM(orders.discount), 0) AS discount,
			IFNULL(SUM(orders.taxes), 0) AS tax,
			IFNULL(SUM(orders.grand_total), 0) AS total
		FROM orders
		WHERE %s
		GROUP BY orders.id
		ORDER BY MAX(orders.created_at) DESC
		LIMIT ? OFFSET ?`, whereStmt)

	dataArgs := append(args, limit, offset)

	rows, err := db.QueryContext(c, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn báo cáo doanh số bán hàng: %v", err)
	}
	defer rows.Close()

	// 6. Map kết quả về []map[string]interface{}
	items := make([]map[string]interface{}, 0)

	for rows.Next() {
		var id int64
		var startDate string
		var endDate string
		var totalOrders int64
		var totalProducts float64
		var subTotal float64
		var shippingCost float64
		var discount float64
		var tax float64
		var total float64

		err := rows.Scan(
			&id,
			&startDate,
			&endDate,
			&totalOrders,
			&totalProducts,
			&subTotal,
			&shippingCost,
			&discount,
			&tax,
			&total,
		)
		if err != nil {
			return nil, err
		}

		itemMap := map[string]interface{}{
			"id":             id,
			"start_date":     startDate,
			"end_date":       endDate,
			"total_orders":   totalOrders,
			"total_products": totalProducts,
			"sub_total":      subTotal,
			"shipping_cost":  shippingCost,
			"discount":       discount,
			"tax":            tax,
			"total":          total,
		}

		items = append(items, itemMap)
	}

	// 7. Gọi helper build pagination chuẩn Laravel (ép kiểu totalCount về int)
	response := utils.BuildLaravelPagination(c, items, int(totalCount), page, limit)
	return response, nil
}

func GetCouponsReport(c *gin.Context, filter models.CouponReportFilter) (interface{}, error) {
	// 1. Lấy DB Connection từ Context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể lấy kết nối DB từ context: %v", err)
	}

	// 2. Xử lý giá trị mặc định cho Phân trang
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 15 // Mặc định như setting('elements_per_page')
	}

	// 3. Dựng câu điều kiện WHERE động (Mặc định lấy đơn hàng có coupon_id IS NOT NULL)
	whereClauses := []string{"orders.coupon_id IS NOT NULL"}
	args := []interface{}{}

	if filter.CouponName != "" {
		whereClauses = append(whereClauses, "coupons.name LIKE ?")
		args = append(args, "%"+filter.CouponName+"%")
	}

	if filter.CouponCode != "" {
		whereClauses = append(whereClauses, "coupons.code LIKE ?")
		args = append(args, "%"+filter.CouponCode+"%")
	}

	if filter.StartDate != "" {
		whereClauses = append(whereClauses, "DATE(coupons.created_at) >= ?")
		args = append(args, filter.StartDate)
	}

	if filter.EndDate != "" {
		whereClauses = append(whereClauses, "DATE(coupons.created_at) <= ?")
		args = append(args, filter.EndDate)
	}

	whereStmt := strings.Join(whereClauses, " AND ")

	// 4. Đếm tổng số nhóm coupon_id (Total Count) cho Phân trang
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT orders.coupon_id 
			FROM orders
			LEFT JOIN coupons ON coupons.id = orders.coupon_id
			WHERE %s 
			GROUP BY orders.coupon_id
		) AS total_groups`, whereStmt)

	var totalCount int64
	err = db.QueryRowContext(c, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("lỗi đếm tổng số bản ghi báo cáo mã giảm giá: %v", err)
	}

	// 5. Query lấy dữ liệu thống kê mã giảm giá
	offset := (page - 1) * limit
	dataQuery := fmt.Sprintf(`
		SELECT 
			orders.coupon_id,
			IFNULL(coupons.code, '') AS code,
			IFNULL(coupons.name, '') AS name,
			MIN(orders.created_at) AS start_date,
			MAX(orders.created_at) AS end_date,
			COUNT(*) AS total_orders,
			IFNULL(SUM(orders.item_count), 0) AS total_products,
			IFNULL(SUM(orders.grand_total), 0) AS total
		FROM orders
		LEFT JOIN coupons ON coupons.id = orders.coupon_id
		WHERE %s
		GROUP BY orders.coupon_id, coupons.code, coupons.name
		ORDER BY MAX(orders.created_at) DESC
		LIMIT ? OFFSET ?`, whereStmt)

	dataArgs := append(args, limit, offset)

	rows, err := db.QueryContext(c, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn báo cáo mã giảm giá: %v", err)
	}
	defer rows.Close()

	// 6. Map kết quả về []map[string]interface{}
	items := make([]map[string]interface{}, 0)

	for rows.Next() {
		var couponID interface{}
		var code string
		var name string
		var startDate string
		var endDate string
		var totalOrders int64
		var totalProducts float64
		var total float64

		err := rows.Scan(
			&couponID,
			&code,
			&name,
			&startDate,
			&endDate,
			&totalOrders,
			&totalProducts,
			&total,
		)
		if err != nil {
			return nil, err
		}

		itemMap := map[string]interface{}{
			"coupon_id":      couponID,
			"code":           code,
			"name":           name,
			"start_date":     startDate,
			"end_date":       endDate,
			"total_orders":   totalOrders,
			"total_products": totalProducts,
			"total":          total,
		}

		items = append(items, itemMap)
	}

	// 7. Gọi helper build pagination chuẩn Laravel (ép kiểu totalCount về int)
	response := utils.BuildLaravelPagination(c, items, int(totalCount), page, limit)
	return response, nil
}
