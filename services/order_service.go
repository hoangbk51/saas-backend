package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func SaveOrderFromCart(c *gin.Context, tx *sql.Tx, req models.OrderRequest, cartData *models.RecalculateResult) (*models.Order, error) {
	// 1. Chuẩn bị địa chỉ Shipping an toàn
	shipping := req.Billing
	if req.OtherAddress && req.Shipping != nil {
		shipping = req.Shipping
	}

	// Trích xuất an toàn các trường Billing
	bFirst := getMapString(req.Billing, "first_name")
	bLast := getMapString(req.Billing, "last_name")
	bAddr1 := getMapString(req.Billing, "address_1")
	bDistrict := getMapString(req.Billing, "district_id")
	bState := getMapString(req.Billing, "state_id")
	bZip := getMapString(req.Billing, "zip")
	bCountry := getMapString(req.Billing, "country_id")

	// Trích xuất an toàn các trường Shipping
	sFirst := getMapString(shipping, "first_name")
	sLast := getMapString(shipping, "last_name")
	sAddr1 := getMapString(shipping, "address_1")
	sDistrict := getMapString(shipping, "district_id")
	sState := getMapString(shipping, "state_id")
	sZip := getMapString(shipping, "zip")
	sCountry := getMapString(shipping, "country_id")

	// 2. Tạo Order Number & Customer ID
	orderNumber := fmt.Sprintf("ORD-%d", time.Now().UnixNano())

	var customerID int
	if id, exists := c.Get("customerID"); exists {
		customerID = id.(int)
	} else {
		customerID = 0 // Guest
	}

	utils.LogSQL("start insert order")

	// 3. Insert vào bảng orders
	queryOrder := `
		INSERT INTO orders (
			order_number, customer_id, item_count, quantity, grand_total,
			billing_first_name, billing_last_name, billing_address1, billing_city, billing_state, billing_zip, billing_country,
			shipping_first_name, shipping_last_name, shipping_address1, shipping_city, shipping_state, shipping_zip, shipping_country,
			email, note, customer_phone, payment_method_code, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	res, err := tx.Exec(queryOrder,
		orderNumber, customerID, len(cartData.Items), cartData.Count, cartData.Total,
		bFirst, bLast, bAddr1, bDistrict, bState, bZip, bCountry,
		sFirst, sLast, sAddr1, sDistrict, sState, sZip, sCountry,
		req.CustomerEmail, req.Note, req.CustomerPhone, req.PaymentMethod,
	)
	if err != nil {
		return nil, fmt.Errorf("lỗi insert orders: %v", err)
	}
	utils.LogSQL("finish insert order id= ")
	orderID, _ := res.LastInsertId()
	utils.LogToFile("Tạo đơn hàng thành công với Order ID = %v", orderID)

	// 4. Insert Order Items & Cập nhật kho
	for _, item := range cartData.Items {
		optionJSON, err := json.Marshal(item.Option)
		if err != nil {
			optionJSON = []byte("[]")
		}

		_, err = tx.Exec(`
			INSERT INTO order_items (
				order_id, product_id, product_variant_id, item_description, `+"`option`"+`, quantity, unit_price, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
			orderID,
			item.ProductID,
			item.ProductVariantID,
			item.ItemDescription,
			string(optionJSON),
			item.Quantity,
			item.UnitPrice,
		)

		if err != nil {
			return nil, fmt.Errorf("lỗi insert order_items: %v", err)
		}

		// 5. Trừ kho
		if item.ProductVariantID != nil && *item.ProductVariantID > 0 {
			_, err = tx.Exec(`UPDATE product_variants SET quantity = quantity - ? WHERE id = ?`,
				item.Quantity, *item.ProductVariantID)
		} else {
			_, err = tx.Exec(`UPDATE products SET stock_quantity = stock_quantity - ? WHERE id = ?`,
				item.Quantity, item.ProductID)
		}
		if err != nil {
			return nil, fmt.Errorf("lỗi update kho: %v", err)
		}
	}
	utils.LogToFile("order items ok với Order ID = %v", orderID)

	// 6. Order Totals
	for i, charge := range cartData.Charges {
		_, err = tx.Exec(`
			INSERT INTO order_totals (order_id, sort_order, code, title, value)
			VALUES (?, ?, ?, ?, ?)`,
			orderID, i+1, charge.Code, charge.Title, charge.Value,
		)
		if err != nil {
			return nil, fmt.Errorf("lỗi insert order_totals: %v", err)
		}
	}

	// 7. Coupon
	if cartData.CouponID != 0 {
		_, err = tx.Exec("UPDATE coupons SET quantity = quantity - 1 WHERE id = ?", cartData.CouponID)
		if err != nil {
			return nil, fmt.Errorf("lỗi update coupon: %v", err)
		}
	}

	return &models.Order{
		BillingFirstName:  bFirst,
		BillingLastName:   bLast,
		ID:                orderID,
		OrderNumber:       orderNumber,
		CustomerPhone:     req.CustomerPhone,
		Email:             req.CustomerEmail,
		GrandTotal:        cartData.Total,
		CustomerID:        req.CustomerID,
		PaymentMethodCode: req.PaymentMethod,
	}, nil
}
func getMapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key]; ok && val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
func UpdateOrderDetail(c *gin.Context, dto models.UpdateOrderDTO) (*models.Order, error) {
	// 1. Tính toán lại giỏ hàng / tổng tiền từ danh sách items mới
	cartData, err := RecalculateCart(c, dto.Items)
	if err != nil {
		return nil, fmt.Errorf("lỗi tính toán giỏ hàng: %v", err)
	}

	// 2. Lấy kết nối DB & Khởi tạo Transaction
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối DB: %v", err)
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return nil, fmt.Errorf("lỗi tạo transaction: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 3. Thực thi cập nhật DB với DTO
	updatedOrder, err := UpdateOrderFromCart(c, tx, dto, cartData)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("lỗi commit transaction: %v", err)
	}

	return updatedOrder, nil
}

func UpdateOrderFromCart(c *gin.Context, tx *sql.Tx, dto models.UpdateOrderDTO, cartData *models.RecalculateResult) (*models.Order, error) {
	// 1. Cập nhật bảng orders
	queryUpdateOrder := `
		UPDATE orders SET 
			billing_first_name = ?, billing_last_name = ?, billing_address1 = ?, 
			billing_city = ?, billing_state = ?, billing_zip = ?, billing_country = ?,
			shipping_first_name = ?, shipping_last_name = ?, shipping_address1 = ?, 
			shipping_city = ?, shipping_state = ?, shipping_zip = ?, shipping_country = ?,
			email = ?, note = ?, customer_phone = ?, payment_method_code = ?, shipping_method_code = ?, 
			grand_total = ?, item_count = ?, quantity = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL`

	_, err := tx.ExecContext(c.Request.Context(), queryUpdateOrder,
		// Billing fields
		dto.BillingFirstName, dto.BillingLastName, dto.BillingAddress1,
		dto.BillingCity, dto.BillingState, dto.BillingZip, dto.BillingCountry,
		// Shipping fields
		dto.ShippingFirstName, dto.ShippingLastName, dto.ShippingAddress1,
		dto.ShippingCity, dto.ShippingState, dto.ShippingZip, dto.ShippingCountry,
		// Info & Totals
		dto.Email, dto.Note, dto.CustomerPhone, dto.PaymentMethod, dto.ShippingMethodCode,
		cartData.Total, len(cartData.Items), cartData.Quantity, dto.OrderID,
	)
	if err != nil {
		return nil, fmt.Errorf("lỗi cập nhật bảng orders: %v", err)
	}

	// 2. Hoàn trả kho cho các items cũ
	rows, err := tx.QueryContext(c.Request.Context(), "SELECT product_id, quantity FROM order_items WHERE order_id = ?", dto.OrderID)
	if err != nil {
		return nil, fmt.Errorf("lỗi lấy danh sách items cũ: %v", err)
	}

	type OldItem struct {
		ProductID int64
		Quantity  int
	}
	var oldItems []OldItem
	for rows.Next() {
		var item OldItem
		if err := rows.Scan(&item.ProductID, &item.Quantity); err == nil {
			oldItems = append(oldItems, item)
		}
	}
	rows.Close()

	for _, oldItem := range oldItems {
		if oldItem.ProductID > 0 {
			_, _ = tx.ExecContext(c.Request.Context(),
				"UPDATE products SET stock_quantity = stock_quantity + ? WHERE id = ?",
				oldItem.Quantity, oldItem.ProductID,
			)
		}
	}

	// 3. Xóa items & totals cũ
	if _, err := tx.ExecContext(c.Request.Context(), "DELETE FROM order_items WHERE order_id = ?", dto.OrderID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(c.Request.Context(), "DELETE FROM order_totals WHERE order_id = ?", dto.OrderID); err != nil {
		return nil, err
	}

	// 4. Insert items mới (Duyệt qua danh sách CartItem)
	for _, item := range cartData.Items {
		_, err = tx.ExecContext(c.Request.Context(), `
			INSERT INTO order_items (
				order_id, product_id, item_description, quantity, unit_price, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
			dto.OrderID, item.ProductID, item.ItemDescription, item.Quantity, item.UnitPrice,
		)
		if err != nil {
			return nil, fmt.Errorf("lỗi thêm order_item mới: %v", err)
		}

		if item.ProductID > 0 {
			_, _ = tx.ExecContext(c.Request.Context(),
				"UPDATE products SET stock_quantity = stock_quantity - ? WHERE id = ?",
				item.Quantity, item.ProductID,
			)
		}
	}

	// 5. Insert order totals mới
	for i, charge := range cartData.Charges {
		_, err = tx.ExecContext(c.Request.Context(), `
			INSERT INTO order_totals (order_id, sort_order, code, title, value)
			VALUES (?, ?, ?, ?, ?)`,
			dto.OrderID, i+1, charge.Code, charge.Title, charge.Value,
		)
		if err != nil {
			return nil, fmt.Errorf("lỗi thêm order_totals mới: %v", err)
		}
	}

	return &models.Order{
		ID:                dto.OrderID,
		CustomerPhone:     dto.CustomerPhone,
		Email:             dto.Email,
		GrandTotal:        cartData.Total,
		PaymentMethodCode: dto.PaymentMethod,
	}, nil
}

// CreateTransaction tạo bản ghi giao dịch chờ thanh toán
func CreateTransaction(tx *sql.Tx, order *models.Order) (*models.Transaction, error) {
	var customerID interface{}
	if order.CustomerID != nil {
		customerID = *order.CustomerID // Lấy giá trị từ con trỏ
	} else {
		customerID = 0 // Hoặc để trống tùy logic nghiệp vụ
	}

	// 1. Tạo mã code duy nhất (Vibe: Short UUID)
	code := "ref" + uuid.New().String()[:8]

	// 2. Query chuẩn cho go-saas
	query := `INSERT INTO transactions (order_id, code, status, amount, payment_method, customer_id) 
              VALUES (?, ?, 0, ?, ?, ?)`
	utils.LogSQL(query,
		order.ID,
		code,
		order.GrandTotal,
		order.PaymentMethodCode,
		customerID)

	res, err := tx.Exec(query,
		order.ID,
		code,
		order.GrandTotal,
		order.PaymentMethodCode,
		customerID, // Lưu ý: Dùng .Int64 nếu CustomerID là sql.NullInt64
	)

	if err != nil {
		return nil, err // Không Rollback ở đây, để hàm gọi nó quyết định
	}

	lastTxID, _ := res.LastInsertId()

	// 3. Trả về con trỏ Struct để đồng bộ với logic provider.Pay
	return &models.Transaction{
		ID:     lastTxID,
		Code:   code,
		Amount: order.GrandTotal,
	}, nil
}

func GetOrderDetail(c *gin.Context, orderID int) (*models.OrderDetailResponse, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	//orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("id đơn hàng không hợp lệ")
	}

	var detail models.OrderDetailResponse

	// Khai báo các biến Nullable để Scan an toàn tuyệt đối từ MySQL
	var (
		customerID, couponID                                                                                               sql.NullInt64
		paymentMethodCode, shippingMethodCode, email, phone                                                                sql.NullString
		billingFirst, billingLast, billingAddr, billingAddr2, billingZipCode                                               sql.NullString
		shippingFirst, shippingLast, shippingAddr, shippingAddr2, shippingZipCode                                          sql.NullString
		orderNumber, trackingCode                                                                                          sql.NullString
		grandTotal                                                                                                         sql.NullFloat64
		orderStatusId                                                                                                      sql.NullInt64
		billingCity, billingCountry, billingState, shippingCity, shippingCountry, shippingState, billingWard, shippingWard *int64
		totalWeight                                                                                                        *float64
	)

	// 1. Lấy thông tin đơn hàng
	orderQuery := `SELECT id, 
                          order_number, 
                          grand_total, 
                          customer_id, 
                          payment_method_code, 
                          shipping_method_code, 
                          coupon_id, 
                          email, 
                          customer_phone, 
                          billing_first_name, 
                          billing_last_name, 
                          billing_address1, 
                          billing_address2, 
                          billing_city,
                          billing_zip,
                          billing_country,
                          billing_state,
						  billing_ward,
                          shipping_first_name, 
                          shipping_last_name, 
                          shipping_address1, 
                          shipping_address2, 
                          shipping_city,
                          shipping_zip,
                          shipping_country,
                          shipping_state,
						  shipping_ward,
						  order_status_id,
						  tracking_id,
						  total_weight
                   FROM orders 
                   WHERE id = ? AND deleted_at IS NULL`

	err = db.QueryRowContext(c.Request.Context(), orderQuery, orderID).Scan(
		&detail.ID,
		&orderNumber,
		&grandTotal,
		&customerID,
		&paymentMethodCode,
		&shippingMethodCode,
		&couponID,
		&email,
		&phone,
		&billingFirst,
		&billingLast,
		&billingAddr,
		&billingAddr2,
		&billingCity,
		&billingZipCode,
		&billingCountry,
		&billingState,
		&billingWard,
		&shippingFirst,
		&shippingLast,
		&shippingAddr,
		&shippingAddr2,
		&shippingCity,
		&shippingZipCode,
		&shippingCountry,
		&shippingState,
		&shippingWard,
		&orderStatusId,
		&trackingCode,
		&totalWeight,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("không tìm thấy đơn hàng hoặc đơn hàng đã bị xóa")
		}
		return nil, fmt.Errorf("lỗi truy vấn đơn hàng: %v", err)
	}

	// Mapping dữ liệu an toàn từ Nullable về Struct Response
	detail.OrderNumber = orderNumber.String
	detail.GrandTotal = grandTotal.Float64
	detail.PaymentMethodCode = paymentMethodCode.String
	detail.ShippingMethodCode = shippingMethodCode.String

	detail.Email = email.String
	detail.CustomerPhone = phone.String
	detail.BillingFirstName = billingFirst.String
	detail.BillingLastName = billingLast.String
	detail.BillingAddress1 = billingAddr.String
	detail.BillingAddress2 = billingAddr2.String
	detail.BillingCity = billingCity
	detail.BillingState = billingState
	detail.BillingCountry = billingCountry
	detail.BillingZip = billingZipCode.String
	detail.BillingWard = billingWard

	detail.ShippingFirstName = shippingFirst.String
	detail.ShippingLastName = shippingLast.String
	detail.ShippingAddress1 = shippingAddr.String
	detail.ShippingAddress2 = shippingAddr2.String
	detail.ShippingState = shippingState
	detail.ShippingCity = shippingCity
	detail.ShippingZip = shippingZipCode.String
	detail.ShippingCountry = shippingCountry
	detail.ShippingWard = shippingWard

	detail.OrderStatusId = orderStatusId.Int64
	if customerID.Valid {
		detail.CustomerID = &customerID.Int64
	}
	if couponID.Valid {
		detail.CouponID = &couponID.Int64
	}
	detail.TotalWeight = *totalWeight
	detail.TrackingCode = &trackingCode.String
	// 2. Lấy danh sách sản phẩm trong đơn hàng (Bổ sung product_variant_id và option)
	itemsQuery := `
    SELECT 
        id, 
        product_id, 
        product_variant_id, 
        COALESCE(item_description, '') as item_description, 
        IFNULL(` + "`option`" + `, '') AS ` + "`option`" + `, 
        quantity, 
        unit_price 
    FROM order_items 
    WHERE order_id = ?`

	itemRows, err := db.QueryContext(c.Request.Context(), itemsQuery, orderID)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn danh sách sản phẩm: %v", err)
	}
	defer itemRows.Close()

	detail.Items = []models.OrderItem{}
	for itemRows.Next() {
		var item models.OrderItem
		var variantID sql.NullInt64
		var optionRaw string

		err := itemRows.Scan(
			&item.ID,
			&item.ProductID,
			&variantID,
			&item.ItemDescription,
			&optionRaw,
			&item.Quantity,
			&item.UnitPrice,
		)
		if err != nil {
			return nil, err
		}

		// Gán ProductVariantID nếu có giá trị
		if variantID.Valid {
			item.ProductVariantID = &variantID.Int64
		}

		// Unmarshal chuỗi JSON trong cột option thành Struct / Slice của Go
		if optionRaw != "" && optionRaw != "[]" {
			_ = json.Unmarshal([]byte(optionRaw), &item.Option)
		} else {
			// Đảm bảo không bị null khi trả về JSON cho Client
			item.Option = []models.OptionItem{}
		}

		detail.Items = append(detail.Items, item)
	}

	// 3. Lấy thông tin tổng hợp chi phí (OrderTotal)
	totalsQuery := `SELECT COALESCE(title, '') as title, value 
                    FROM order_totals 
                    WHERE order_id = ?`

	totalRows, err := db.QueryContext(c.Request.Context(), totalsQuery, orderID)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn tổng chi phí đơn hàng: %v", err)
	}
	defer totalRows.Close()

	detail.Totals = []models.OrderTotal{}
	for totalRows.Next() {
		var total models.OrderTotal
		if err := totalRows.Scan(&total.Title, &total.Value); err != nil {
			return nil, err
		}
		detail.Totals = append(detail.Totals, total)
	}

	// 4. Lấy lịch sử đơn hàng (OrderHistory)
	historyQuery := `SELECT id, order_id, order_status_id, 
                            COALESCE(comment, '') as comment, 
                            notify, 
                            COALESCE(created_at, '') as created_at, 
                            COALESCE(updated_at, '') as updated_at 
                     FROM order_histories 
                     WHERE order_id = ? 
                     ORDER BY id DESC`

	historyRows, err := db.QueryContext(c.Request.Context(), historyQuery, orderID)
	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn lịch sử đơn hàng: %v", err)
	}
	defer historyRows.Close()

	detail.Histories = []models.OrderHistory{}
	for historyRows.Next() {
		var h models.OrderHistory
		var createdAtBytes, updatedAtBytes []byte

		if err := historyRows.Scan(
			&h.ID,
			&h.OrderID,
			&h.OrderStatusID,
			&h.Comment,
			&h.Notify,
			&createdAtBytes,
			&updatedAtBytes,
		); err != nil {
			return nil, err
		}

		if len(createdAtBytes) > 0 {
			h.CreatedAt = string(createdAtBytes)
		}
		if len(updatedAtBytes) > 0 {
			h.UpdatedAt = string(updatedAtBytes)
		}

		detail.Histories = append(detail.Histories, h)
	}

	return &detail, nil
}

func SearchOrdersService(c *gin.Context) (interface{}, int, error) {
	domainApi := c.Request.Host

	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	// 1. Lấy tham số Filter từ Query Params
	id := c.Query("filter[id]")
	customerName := c.Query("filter[customer_name]")
	email := c.Query("filter[email]")
	totalMin := c.Query("filter[total_min]")
	totalMax := c.Query("filter[total_max]")
	fromDate := c.Query("filter[from_date]")
	toDate := c.Query("filter[to_date]")
	paymentMethod := c.Query("filter[payment_method]")
	orderStatus := c.Query("filter[order_status]")
	shippingMethod := c.Query("filter[shipping_method]")

	limit, _ := strconv.Atoi(c.DefaultQuery("filter[limit]", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	sortBy := c.DefaultQuery("sort_by", "id")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	// 2. Query gốc
	query := `
		SELECT 
			o.id, o.order_number, o.grand_total, o.email, o.customer_phone,
			o.billing_first_name, o.billing_last_name, o.payment_method_code, 
			o.shipping_method_code, o.order_status_id as status, o.created_at, o.updated_at
		FROM orders as o 
		WHERE o.deleted_at IS NULL`

	var args []interface{}
	whereClause := ""

	// --- LỌC ĐỘNG (FILTER CLAUSES) ---
	if id != "" {
		whereClause += " AND o.id = ?"
		args = append(args, id)
	}

	if customerName != "" {
		likePattern := "%" + customerName + "%"
		whereClause += " AND CONCAT(IFNULL(o.billing_first_name, ''), ' ', IFNULL(o.billing_last_name, '')) LIKE ?"
		args = append(args, likePattern)
	}

	if email != "" {
		whereClause += " AND o.email LIKE ?"
		args = append(args, "%"+email+"%")
	}

	if totalMin != "" {
		if val, err := strconv.ParseFloat(totalMin, 64); err == nil {
			whereClause += " AND o.grand_total >= ?"
			args = append(args, val)
		}
	}

	if totalMax != "" {
		if val, err := strconv.ParseFloat(totalMax, 64); err == nil {
			whereClause += " AND o.grand_total <= ?"
			args = append(args, val)
		}
	}

	if fromDate != "" {
		whereClause += " AND o.created_at >= ?"
		args = append(args, fromDate+" 00:00:00")
	}

	if toDate != "" {
		whereClause += " AND o.created_at <= ?"
		args = append(args, toDate+" 23:59:59")
	}

	if paymentMethod != "" {
		whereClause += " AND o.payment_method_code = ?"
		args = append(args, paymentMethod)
	}

	if orderStatus != "" {
		whereClause += " AND o.order_status_id = ?"
		args = append(args, orderStatus)
	}

	if shippingMethod != "" {
		whereClause += " AND o.shipping_method_code = ?"
		args = append(args, shippingMethod)
	}

	// --- TÍNH TỔNG SỐ BẢN GHI ---
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM orders as o WHERE o.deleted_at IS NULL " + whereClause
	utils.LogSQL(countQuery, args...)

	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery, args...).Scan(&totalCount)
	if err != nil {
		log.Printf("Count orders error: %v", err)
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}

	query += whereClause

	// --- BẢO MẬT SẮP XẾP ĐỘNG ---
	orderColumn := "o.id"
	orderDirection := "DESC"
	if strings.ToLower(sortOrder) == "asc" {
		orderDirection = "ASC"
	}

	switch sortBy {
	case "id":
		orderColumn = "o.id"
	case "customer_name", "name":
		orderColumn = "o.billing_first_name"
	case "email":
		orderColumn = "o.email"
	case "total", "grand_total":
		orderColumn = "o.grand_total"
	case "date", "created_at":
		orderColumn = "o.created_at"
	case "status", "order_status":
		orderColumn = "o.order_status_id"
	case "payment_method":
		orderColumn = "o.payment_method_code"
	default:
		orderColumn = "o.id"
	}

	query += fmt.Sprintf(" ORDER BY %s %s LIMIT ? OFFSET ?", orderColumn, orderDirection)

	mainArgs := append([]interface{}{}, args...)
	mainArgs = append(mainArgs, limit, offset)

	utils.LogSQL(query, mainArgs...)

	// --- THỰC THI QUERY ---
	rows, err := tenantDB.QueryContext(c.Request.Context(), query, mainArgs...)
	if err != nil {
		log.Printf("Search orders error: %v", err)
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	// --- PARSE DỮ LIỆU ---
	const timeLayout = "2006-01-02 15:04:05"
	var orders []map[string]interface{}

	for rows.Next() {
		o, err := utils.ScanRowToMap(rows)
		if err != nil {
			log.Println("Scan row error:", err)
			continue
		}

		firstName, _ := o["billing_first_name"].(string)
		lastName, _ := o["billing_last_name"].(string)
		o["customer_name"] = strings.TrimSpace(fmt.Sprintf("%s %s", firstName, lastName))

		o["grand_total"] = utils.ParseToFloat(o["grand_total"])

		if createdAt, ok := o["created_at"].(time.Time); ok && !createdAt.IsZero() {
			o["created_at"] = createdAt.Format(timeLayout)
		} else {
			o["created_at"] = nil
		}

		if updatedAt, ok := o["updated_at"].(time.Time); ok && !updatedAt.IsZero() {
			o["updated_at"] = updatedAt.Format(timeLayout)
		} else {
			o["updated_at"] = nil
		}

		o["link"] = fmt.Sprintf("http://%s/admin/orders/%v", domainApi, o["id"])
		orders = append(orders, o)
	}

	// Đóng gói cấu trúc Laravel Pagination
	response := utils.BuildLaravelPagination(c, orders, totalCount, page, limit)
	return response, http.StatusOK, nil
}
