package services

import (
	"database/sql"
	"fmt"
	"go-saas/models"
	"go-saas/shipping"
	_ "go-saas/shipping/providers" // 🟢 Trigger init() của các provider an toàn, không lo import cycle	"go-saas/utils"
	"go-saas/utils"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const optionSubquery = `
	COALESCE(
		(
			SELECT CONCAT('[', GROUP_CONCAT(
				JSON_OBJECT(
					'option_name', o.name,
					'option_value', ov.name
				)
			), ']')
			FROM product_variant_option_values pvov
			JOIN option_values ov ON pvov.option_value_id = ov.id
			JOIN options o ON pvov.option_id = o.id
			WHERE pvov.variant_id = ci.product_variant_id
		), '[]'
	)`

func GetCartById(c *gin.Context, cartID int64) (*models.Cart, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	cart := &models.Cart{}
	query := `SELECT 
				id, customer_id, ip_address, shipping_country_id, shipping_zone_id, 
				shipping_rate_id, item_count, quantity, total, discount, 
				shipping, taxes, grand_total, shipping_weight, COALESCE(billing_address, '') AS billing_address, 
				COALESCE(shipping_address, '') AS shipping_address, coupon_id, payment_method_id, shipping_method_code, shipping_method_sub_code, 
				shipping_state_id, shipping_district_id,shipping_ward_id, created_at, updated_at 
			  FROM carts WHERE id = ? AND deleted_at IS NULL LIMIT 1`

	err = db.QueryRow(query, cartID).Scan(
		&cart.ID, &cart.CustomerID, &cart.IPAddress, &cart.ShippingCountryID, &cart.ShippingZoneID,
		&cart.ShippingRateID, &cart.ItemCount, &cart.Quantity, &cart.Total, &cart.Discount,
		&cart.Shipping, &cart.Taxes, &cart.GrandTotal, &cart.ShippingWeight, &cart.BillingAddress,
		&cart.ShippingAddress, &cart.CouponID, &cart.PaymentMethodID, &cart.ShippingMethodCode, &cart.ShippingMethodSubCode,
		&cart.ShippingStateID, &cart.ShippingDistrictID, &cart.ShippingWardID, &cart.CreatedAt, &cart.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Bọc `option_json` bằng backtick hoặc dùng tên biến không đụng reserved word
	itemQuery := fmt.Sprintf(`
		SELECT 
			ci.id, 
			ci.product_id, 
			ci.product_variant_id, 
			ci.item_description, 
			ci.quantity, 
			ci.unit_price,
			%s AS `+"`option_json`"+`
		FROM cart_items ci 
		WHERE ci.cart_id = ? AND ci.deleted_at IS NULL`, optionSubquery)

	rows, err := db.Query(itemQuery, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.CartItem
		if err := rows.Scan(
			&item.ID,
			&item.ProductID,
			&item.ProductVariantID,
			&item.ItemDescription,
			&item.Quantity,
			&item.UnitPrice,
			&item.Option,
		); err != nil {
			return nil, err
		}
		cart.Items = append(cart.Items, item)
	}

	return cart, nil
}

func getCartItemsWithProduct(c *gin.Context, cartID int64) ([]models.CartItemWithProduct, error) {
	db, _ := utils.GetDBFromContext(c)

	query := fmt.Sprintf(`
		SELECT 
			ci.id, 
			ci.product_id, 
			ci.product_variant_id, 
			ci.quantity, 
			ci.unit_price, 
			ci.item_description,
			p.title AS title, 
			COALESCE(p.shipping_weight, 0) AS weight,
			COALESCE(
				(
					SELECT GROUP_CONCAT(cp.category_id) 
					FROM category_product cp 
					WHERE cp.product_id = ci.product_id
				), ''
			) AS category_ids_str,
			%s AS `+"`option`"+`
		FROM cart_items ci
		JOIN products p ON ci.product_id = p.id
		WHERE ci.cart_id = ? AND ci.deleted_at IS NULL`, optionSubquery)

	rows, err := db.QueryContext(c.Request.Context(), query, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.CartItemWithProduct
	for rows.Next() {
		var i models.CartItemWithProduct
		var catIDsStr string

		err := rows.Scan(
			&i.ID,
			&i.ProductID,
			&i.ProductVariantID,
			&i.Quantity,
			&i.UnitPrice,
			&i.ItemDescription,
			&i.ProductTitle,
			&i.ProductWeight,
			&catIDsStr, // 🟢 Scan chuỗi Category IDs (vd: "1,4,12")
			&i.Option,
		)
		if err != nil {
			return nil, err
		}

		// Convert chuỗi Category IDs thành slice []int64
		if catIDsStr != "" {
			for _, strID := range strings.Split(catIDsStr, ",") {
				if id, err := strconv.ParseInt(strings.TrimSpace(strID), 10, 64); err == nil {
					i.CategoryIDs = append(i.CategoryIDs, id)
				}
			}
		}

		results = append(results, i)
	}
	return results, nil
}

func CartRecalculate(c *gin.Context, cart *models.Cart) (*models.RecalculateResult, error) {
	utils.LogToFile("CartRecalculate")
	cartID := cart.ID
	db, _ := utils.GetDBFromContext(c)

	cartItems, err := getCartItemsWithProduct(c, cartID)
	if err != nil {
		return nil, err
	}

	var count int
	var subTotal float64
	var totalWeight float64
	items := make([]models.CartItemResult, 0, len(cartItems))

	for _, item := range cartItems {
		qty := item.Quantity
		unitPrice := item.UnitPrice
		lineTotal := unitPrice * float64(qty)

		count += qty
		subTotal += lineTotal
		totalWeight += item.ProductWeight * float64(qty)

		var variantID *int64
		if item.ProductVariantID.Valid {
			v := item.ProductVariantID.Int64
			variantID = &v
		}

		parsedOptions := utils.ParseOptions(item.Option)

		items = append(items, models.CartItemResult{
			ID:               item.ID,
			ProductID:        item.ProductID,
			ProductVariantID: variantID,
			ItemDescription:  item.ProductTitle,
			Quantity:         qty,
			UnitPrice:        unitPrice,
			Option:           parsedOptions,
		})
	}

	// Cập nhật weight cho Cart
	cart.ShippingWeight = totalWeight

	// 1. Lấy danh sách Shipping Methods khả dụng
	availableShippingMethods := GetAvailableShippingMethods(c, cart)

	// 2. Khởi tạo danh sách Charges
	charges := []models.CartCharge{
		{Title: "Sub total", Value: subTotal, Code: "sub_total"},
	}
	runningTotal := subTotal

	// 3. Tính Thuế
	taxList := calculateTaxes(c, cartID, subTotal)
	for _, tax := range taxList {
		runningTotal += tax.Value
		charges = append(charges, tax)
	}

	// 4. Lấy Phí vận chuyển
	shippingCost := calculateShippingRate(cart, availableShippingMethods)
	cart.Shipping = &shippingCost
	charges = append(charges, models.CartCharge{Title: "Shipping", Value: shippingCost, Code: "shipping"})
	runningTotal += shippingCost

	// 5. Tính Discount / Coupon (Truyền danh sách cartItems để kiểm tra sản phẩm/danh mục áp dụng)
	discount := calculateDiscount(c, cart.CouponID, subTotal, cartItems)
	if discount > 0 {
		charges = append(charges, models.CartCharge{Title: "Discount", Value: discount, Code: "discount"})
		runningTotal -= discount
	}

	// 6. Cập nhật DB Cart
	_, err = db.ExecContext(c.Request.Context(), `
		UPDATE carts 
		SET item_count = ?, quantity = ?, total = ?, discount = ?, grand_total = ?, shipping_weight = ?, shipping = ?, updated_at = NOW() 
		WHERE id = ?`,
		len(items), count, subTotal, discount, runningTotal, totalWeight, shippingCost, cartID,
	)
	if err != nil {
		return nil, err
	}

	return &models.RecalculateResult{
		Count:           count,
		Quantity:        count,
		Items:           items,
		Total:           runningTotal,
		Charges:         charges,
		ShippingMethods: availableShippingMethods,
		Data:            cart,
	}, nil
}

func calculateTaxes(c *gin.Context, cartID int64, subTotal float64) []models.CartCharge {
	// Logic: Query bảng tax_rules dựa trên shipping_country_id và shipping_state_id của Cart
	// Tương tự hàm taxes() trong PHP của bạn
	return []models.CartCharge{}
}

func calculateShippingRate(cart *models.Cart, availableMethods []models.ShippingMethod) float64 {
	// Nếu cart chưa chọn phương thức vận chuyển
	if cart.ShippingMethodCode == nil || *cart.ShippingMethodCode == "" {
		return 0
	}

	selectedCode := *cart.ShippingMethodCode

	// Tìm phương thức vận chuyển tương ứng trong danh sách đã lấy trước đó
	for _, method := range availableMethods {
		if method.Code == selectedCode {
			// 1. Trường hợp phương thức có dịch vụ con (ViettelPost, GHN, GHTK...)
			if len(method.Services) > 0 {
				// Ưu tiên khớp theo sub_code nếu người dùng đã chọn gói dịch vụ cụ thể
				if cart.ShippingMethodSubCode != nil && *cart.ShippingMethodSubCode != "" {
					subCode := *cart.ShippingMethodSubCode
					for _, service := range method.Services {
						if service.Code == subCode {
							return service.Value // 🎯 Trả về giá của sub-service trùng khớp
						}
					}
				}

				// Nếu không chọn sub_code hoặc không khớp, lấy dịch vụ rẻ nhất (đã sort)
				return method.Services[0].Value
			}

			// 2. Trường hợp phương thức cố định không có sub-services (Flat Rate, Free Shipping...)
			return method.Value
		}
	}

	return 0
}

// Helper parse thời gian từ *string an toàn
func parseTimeString(timeStr *string) *time.Time {
	if timeStr == nil || *timeStr == "" {
		return nil
	}

	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, *timeStr); err == nil {
			return &t
		}
	}

	return nil
}

func calculateDiscount(c *gin.Context, couponID *int64, subTotal float64, cartItems []models.CartItemWithProduct) float64 {
	if couponID == nil || *couponID == 0 {
		return 0
	}

	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0
	}

	ctx := c.Request.Context()

	// 1. Lấy thông tin coupon
	var cp models.Coupon
	var startTimeStr, endTimeStr sql.NullString

	query := `SELECT id, type, value, min_order_amount, quantity, active, starting_time, ending_time 
			  FROM coupons WHERE id = ? AND deleted_at IS NULL LIMIT 1`

	err = db.QueryRowContext(ctx, query, couponID).Scan(
		&cp.ID,
		&cp.Type,
		&cp.Value,
		&cp.MinOrderAmount,
		&cp.Quantity,
		&cp.Active,
		&startTimeStr,
		&endTimeStr,
	)
	if err != nil {
		return 0
	}

	if startTimeStr.Valid {
		cp.StartingTime = &startTimeStr.String
	}
	if endTimeStr.Valid {
		cp.EndingTime = &endTimeStr.String
	}

	// 2. Validate cơ bản
	now := time.Now()
	if cp.Active != nil && !*cp.Active {
		return 0
	}
	if cp.Quantity != nil && *cp.Quantity <= 0 {
		return 0
	}

	startTime := parseTimeString(cp.StartingTime)
	if startTime != nil && now.Before(*startTime) {
		return 0
	}

	endTime := parseTimeString(cp.EndingTime)
	if endTime != nil && now.After(*endTime) {
		return 0
	}

	// 3. Lấy danh sách Product IDs & Category IDs áp dụng cho Coupon
	allowedProductIDs := make(map[int64]bool)
	rowsP, err := db.QueryContext(ctx, "SELECT product_id FROM coupon_products WHERE coupon_id = ?", *couponID)
	if err == nil {
		defer rowsP.Close()
		for rowsP.Next() {
			var pID int64
			if err := rowsP.Scan(&pID); err == nil {
				allowedProductIDs[pID] = true
			}
		}
	}

	allowedCategoryIDs := make(map[int64]bool)
	rowsC, err := db.QueryContext(ctx, "SELECT category_id FROM coupon_categories WHERE coupon_id = ?", *couponID)
	if err == nil {
		defer rowsC.Close()
		for rowsC.Next() {
			var cID int64
			if err := rowsC.Scan(&cID); err == nil {
				allowedCategoryIDs[cID] = true
			}
		}
	}

	hasProductRestriction := len(allowedProductIDs) > 0
	hasCategoryRestriction := len(allowedCategoryIDs) > 0

	// 4. Lọc và tính tổng tiền các sản phẩm hợp lệ (Eligible Total)
	var eligibleSubTotal float64

	for _, item := range cartItems {
		itemTotal := item.UnitPrice * float64(item.Quantity)

		if hasProductRestriction || hasCategoryRestriction {
			isProductValid := hasProductRestriction && allowedProductIDs[item.ProductID]

			// Kiểm tra xem sản phẩm có danh mục nào trùng với danh mục coupon cho phép không
			isCategoryValid := false
			if hasCategoryRestriction {
				for _, catID := range item.CategoryIDs {
					if allowedCategoryIDs[catID] {
						isCategoryValid = true
						break
					}
				}
			}

			if isProductValid || isCategoryValid {
				eligibleSubTotal += itemTotal
			}
		} else {
			// Coupon áp dụng toàn bộ cửa hàng
			eligibleSubTotal += itemTotal
		}
	}

	// Nếu không có sản phẩm nào đủ điều kiện hoặc tổng tiền < min_order_amount
	if eligibleSubTotal <= 0 || eligibleSubTotal < cp.MinOrderAmount {
		return 0
	}

	// 5. Tính giá trị giảm giá
	var discountAmount float64

	if cp.Type == "percent" {
		discountAmount = (eligibleSubTotal * cp.Value) / 100
	} else {
		discountAmount = cp.Value
	}

	if discountAmount > eligibleSubTotal {
		discountAmount = eligibleSubTotal
	}

	return discountAmount
}

func GetCart(c *gin.Context, clientIP string) (*models.RecalculateResult, error) {
	utils.LogToFile("GetCart")

	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	var cartID int64
	var query string
	var args []any

	// 1. Kiểm tra nếu có customerID trong Gin Context (Đã đăng nhập)
	if customerID, exists := c.Get("customerID"); exists && customerID != nil {
		query = `SELECT id FROM carts WHERE customer_id = ? AND deleted_at IS NULL LIMIT 1`
		args = append(args, customerID)
	} else {
		// 2. Nếu là Guest (Chưa đăng nhập), tìm theo IP address
		query = `SELECT id FROM carts WHERE customer_id IS NULL AND ip_address = ? AND deleted_at IS NULL LIMIT 1`
		args = append(args, clientIP)
	}

	// 3. Query lấy cartID
	err = db.QueryRow(query, args...).Scan(&cartID)
	if err != nil {
		return nil, err // Trả về lỗi (ví dụ: sql.ErrNoRows nếu chưa có giỏ hàng)
	}

	// 4. Lấy Full Object Cart
	cart, err := GetCartById(c, cartID)
	if err != nil {
		return nil, err
	}

	utils.LogToFile("debug")

	// 5. Tính toán lại toàn bộ giỏ hàng và trả về kết quả
	return CartRecalculate(c, cart)
}

func UpdateItemQuantity(c *gin.Context, itemID int64, newQty int) (*models.RecalculateResult, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}
	utils.LogSQL(fmt.Sprintf("DEBUG: ItemID nhận được là %d", itemID))
	// 1. Tìm cart_id của item này trước khi update (để recalculate sau đó)
	var cartID int64
	err = db.QueryRow("SELECT cart_id FROM cart_items WHERE id = ? AND deleted_at IS NULL", itemID).Scan(&cartID)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy sản phẩm trong giỏ: %v", err)
	}

	// 2. Nếu số lượng <= 0, xóa item đó (Soft Delete)
	if newQty <= 0 {
		_, err = db.Exec("UPDATE cart_items SET deleted_at = NOW() WHERE id = ?", itemID)
	} else {
		// Cập nhật số lượng mới
		_, err = db.Exec("UPDATE cart_items SET quantity = ?, updated_at = NOW() WHERE id = ?", newQty, itemID)
	}

	if err != nil {
		return nil, fmt.Errorf("lỗi cập nhật database: %v", err)
	}

	// 3. Lấy lại thông tin Cart đầy đủ để truyền vào hàm Recalculate
	cart, err := GetCartById(c, cartID)
	if err != nil {
		return nil, err
	}

	// 4. Tính toán lại toàn bộ giỏ hàng và trả về kết quả mới nhất cho Frontend
	return CartRecalculate(c, cart)
}

func RemoveItemFromCart(c *gin.Context, itemID int64) (*models.RecalculateResult, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	// 1. Tìm cart_id của item này trước khi xóa (để recalculate)
	var cartID int64
	err = db.QueryRow("SELECT cart_id FROM cart_items WHERE id = ? AND deleted_at IS NULL", itemID).Scan(&cartID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sản phẩm không tồn tại trong giỏ hàng")
		}
		return nil, err
	}

	// 2. Xóa sản phẩm (Sử dụng Soft Delete nếu database của bạn có cột deleted_at)
	_, err = db.Exec("UPDATE cart_items SET deleted_at = NOW() WHERE id = ?", itemID)
	// Nếu bạn dùng Hard Delete (xóa hẳn): "DELETE FROM cart_items WHERE id = ?"

	if err != nil {
		return nil, fmt.Errorf("lỗi khi xóa sản phẩm: %v", err)
	}

	// 3. Lấy lại thông tin Cart và tính toán lại toàn bộ (Total, Shipping, Tax...)
	cart, err := GetCartById(c, cartID)
	if err != nil {
		return nil, err
	}

	// 4. Trả về kết quả sau khi đã tính lại tiền
	return CartRecalculate(c, cart)
}

func UpdateCartAddress(c *gin.Context, input models.AddressUpdatePayload, clientIP string) (*models.RecalculateResult, interface{}, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, nil, err
	}

	// 1. Tìm CartID
	var cartID int64
	if customerID, exists := c.Get("customerID"); exists && customerID != nil {
		utils.LogToFile("customerID= %d", customerID)
		err = db.QueryRow("SELECT id FROM carts WHERE customer_id = ? AND deleted_at IS NULL LIMIT 1", customerID).Scan(&cartID)

	} else {
		err = db.QueryRow("SELECT id FROM carts WHERE ip_address = ? AND customer_id IS NULL AND deleted_at IS NULL LIMIT 1", clientIP).Scan(&cartID)

	}
	if err != nil {
		return nil, nil, fmt.Errorf("không tìm thấy giỏ hàng")
	}
	utils.LogToFile("cartID")

	utils.LogToFile(cartID)
	// 2. Cập nhật cả Billing và Shipping vào DB
	query := `UPDATE carts SET 
                shipping_country_id = ?, shipping_state_id = ?, shipping_district_id = ?, shipping_ward_id = ?,
                updated_at = NOW() 
              WHERE id = ?`
	utils.LogSQL(query,
		input.Shipping.CountryID, input.Shipping.StateID, input.Shipping.DistrictID, input.Shipping.WardID,
		cartID)

	_, err = db.Exec(query,
		input.Shipping.CountryID, input.Shipping.StateID, input.Shipping.DistrictID, input.Shipping.WardID, // Placeholder cho billing_address string
		cartID,
	)

	// 3. Lấy lại Cart và Recalculate
	cart, _ := GetCartById(c, cartID)

	// Giả sử bạn có hàm này để lấy danh sách hãng vận chuyển khả dụng
	shippingMethods := GetAvailableShippingMethods(c, cart)

	recalc, err := CartRecalculate(c, cart)
	return recalc, shippingMethods, err
}

func CartUpdate(c *gin.Context, input models.CartUpdate, clientIP string) (*models.RecalculateResult, interface{}, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, nil, err
	}

	// 1. Tìm CartID
	var cartID int64
	if customerID, exists := c.Get("customerID"); exists && customerID != nil {
		utils.LogToFile("customerID= %d", customerID)
		err = db.QueryRow("SELECT id FROM carts WHERE customer_id = ? AND deleted_at IS NULL LIMIT 1", customerID).Scan(&cartID)

	} else {
		err = db.QueryRow("SELECT id FROM carts WHERE ip_address = ? AND customer_id IS NULL AND deleted_at IS NULL LIMIT 1", clientIP).Scan(&cartID)

	}
	if err != nil {
		return nil, nil, fmt.Errorf("không tìm thấy giỏ hàng")
	}
	utils.LogToFile("cartID")

	utils.LogToFile(cartID)
	// 2. Cập nhật cả Billing và Shipping vào DB
	query := `UPDATE carts SET shipping_method_code = ? ,shipping_method_sub_code = ? ,
                shipping_country_id = ?, shipping_state_id = ?, shipping_district_id = ?, shipping_ward_id = ?,
                updated_at = NOW() 
              WHERE id = ?`
	utils.LogSQL(query, input.ShippingMethodCode, input.ShippingMethodSubCode,
		input.Shipping.CountryID, input.Shipping.StateID, input.Shipping.DistrictID, input.Shipping.WardID,
		cartID)

	_, err = db.Exec(query, input.ShippingMethodCode, input.ShippingMethodSubCode,
		input.Shipping.CountryID, input.Shipping.StateID, input.Shipping.DistrictID, input.Shipping.WardID, // Placeholder cho billing_address string
		cartID,
	)

	// 3. Lấy lại Cart và Recalculate
	cart, _ := GetCartById(c, cartID)

	// Giả sử bạn có hàm này để lấy danh sách hãng vận chuyển khả dụng
	shippingMethods := GetAvailableShippingMethods(c, cart)

	recalc, err := CartRecalculate(c, cart)
	return recalc, shippingMethods, err
}

func GetAvailableShippingMethods(c *gin.Context, cart *models.Cart) []models.ShippingMethod {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return []models.ShippingMethod{}
	}

	// 0. Đọc các Cấu hình từ Settings DB/Context
	showSubServices := utils.GetSettingInt(c, "shipping_show_sub_services", 1)               // 1: Hiện sub-services, 0: Tự chọn gói rẻ nhất
	selectCheapestProvider := utils.GetSettingInt(c, "shipping_select_cheapest_provider", 0) // 1: Lấy provider rẻ nhất, 0: Show tất cả

	// 1. Query danh sách Shipping Methods đang BẬT (status = 1) từ DB
	var dbMethods []models.DBShippingMethod
	queryDB := `
		SELECT id, code, name 
		FROM shipping_methods 
		WHERE enabled = 1 
	`
	err = db.Select(&dbMethods, queryDB)
	if err != nil || len(dbMethods) == 0 {
		utils.LogToFile(fmt.Sprintf("[Shipping] No active shipping methods found in DB or Error: %v", err))
		return []models.ShippingMethod{}
	}

	shippingMethods := make([]models.ShippingMethod, 0, len(dbMethods))
	utils.LogToFile("shippingMethods")

	utils.LogToFile(shippingMethods)
	// 2. Lặp qua từng phương thức được phép hoạt động trong DB
	for _, dbMethod := range dbMethods {
		option := models.ShippingMethod{
			Code:  dbMethod.Code,
			Label: dbMethod.Name,
		}

		switch dbMethod.Code {

		// --- TH 1: Bảng giá theo Zone & Cân nặng ---
		case "table_rate", "zone_rate":
			zones := getAvailableZones(c, cart.ShippingCountryID, cart.ShippingStateID)
			totalWeight := cart.ShippingWeight
			var rateFound bool

			for _, zone := range zones {
				var res []models.RateQueryResult
				queryRate := `
					SELECT c.name, c.code, srt.rate
					FROM zones AS z 
					JOIN shipping_rate_tables AS srt ON srt.zone_id = z.id  
					JOIN shipping_rate_conditions AS src ON srt.shipping_rate_condition_id = src.id  
					JOIN carriers AS c ON c.id = srt.carrier_id  
					WHERE (z.id = 0 OR z.id = ?) 
					  AND src.min <= ? 
					  AND (src.max IS NULL OR src.max = 0 OR src.max >= ?)
					  AND z.deleted_at IS NULL
				`
				if errRate := db.Select(&res, queryRate, zone.ZoneID, totalWeight, totalWeight); errRate == nil && len(res) > 0 {
					rateFound = true
					option.Value = res[0].Rate
					option.Services = []models.ShippingService{
						{Code: dbMethod.Code + "_standard", Name: dbMethod.Name, Value: res[0].Rate},
					}
					break
				}
			}
			if !rateFound {
				continue // Không tìm thấy rate thỏa mãn zone/weight thì bỏ qua method này
			}

		// --- TH 2: Phí Đồng Giá ---
		case "flat_rate":
			flatFee := utils.GetSettingFloat(c, "flat_rate_price", 30000.0)
			option.Value = flatFee
			option.Services = []models.ShippingService{
				{Code: "flat_rate_std", Name: "Giao hàng tiêu chuẩn", Value: flatFee},
			}

		// --- TH 3: Miễn Phí Vận Chuyển ---
		case "free_shipping":
			option.Value = 0
			option.Services = []models.ShippingService{
				{Code: "free_shipping_std", Name: "Miễn phí giao hàng", Value: 0},
			}

		// --- TH 4: Các Hãng Vận Chuyển API Động (GHN, ViettelPost, GHTK,...) ---
		default:
			utils.LogToFile(dbMethod.Code)
			if provider, ok := shipping.Get(dbMethod.Code); ok {
				utils.LogToFile("ok")

				servicesList, errServices := provider.GetServices(c, cart)
				if errServices != nil || len(servicesList) == 0 {
					// Fallback qua CalculateFee
					fee, errFee := provider.CalculateFee(c, cart)
					if errFee != nil {
						option.Error = "Khu vực không được hỗ trợ"
						option.Value = -1
					} else {
						option.Value = fee
						option.Services = []models.ShippingService{
							{Code: dbMethod.Code + "_std", Name: dbMethod.Name, Value: fee},
						}
					}
				} else {
					// Sắp xếp danh sách sub-services theo giá tăng dần
					sort.Slice(servicesList, func(i, j int) bool {
						return servicesList[i].Value < servicesList[j].Value
					})

					option.Value = servicesList[0].Value

					// Xử lý Setting: Hiển thị hoặc ẩn sub-services
					if showSubServices == 1 {
						option.Services = servicesList
					} else {
						option.Services = []models.ShippingService{} // Ẩn danh sách gói cước con
					}
				}
			} else {
				utils.LogToFile("not ok")
				option.Error = "Chưa cấu hình tích hợp hãng vận chuyển này"
				option.Value = -1
			}
		}

		// Bỏ qua phương thức nếu bị lỗi khu vực/không khả dụng
		if option.Value >= 0 {
			shippingMethods = append(shippingMethods, option)
		}
	}

	if len(shippingMethods) == 0 {
		return shippingMethods
	}

	// Xử lý Setting: Chỉ lấy duy nhất 1 Provider có giá nhỏ nhất
	if selectCheapestProvider == 1 {
		sort.Slice(shippingMethods, func(i, j int) bool {
			return shippingMethods[i].Value < shippingMethods[j].Value
		})
		return []models.ShippingMethod{shippingMethods[0]}
	}

	return shippingMethods
}

// getAvailableZones Sửa truy vấn Zone chuẩn theo bảng zone_detail
func getAvailableZones(c *gin.Context, countryID *int64, stateID *int64) []models.ZoneResult {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return []models.ZoneResult{}
	}

	var zones []models.ZoneResult
	query := `
		SELECT DISTINCT zone_id 
		FROM zone_detail 
		WHERE deleted_at IS NULL 
		  AND ((country_id = ? AND country_id > 0) OR (state_id = ? AND state_id > 0))
	`

	_ = db.Select(&zones, query, countryID, stateID)
	return zones
}

// RecalculateCart hỗ trợ tính toán lại tiền hàng, thuế, phí ship từ danh sách CartItem thuần (Dùng cho Admin Edit Order)
func RecalculateCart(c *gin.Context, items []models.CartItem) (*models.RecalculateResult, error) {
	var result models.RecalculateResult
	var total float64 = 0
	var totalQty int = 0

	for _, item := range items {
		// Nếu bạn lấy giá từ DB:
		// unitPrice := dbProduct.Price

		// Hoặc nếu tin tưởng giá từ dto.Items truyền vào:
		unitPrice := item.UnitPrice

		itemResult := models.CartItemResult{
			ProductID:       item.ProductID,
			ItemDescription: item.ItemDescription,
			Quantity:        item.Quantity,
			UnitPrice:       unitPrice, // 🟢 BẮT BUỘC gán UnitPrice ở đây!
		}

		total += unitPrice * float64(item.Quantity)
		totalQty += item.Quantity
		result.Items = append(result.Items, itemResult)
	}

	result.Total = total
	result.Quantity = totalQty
	return &result, nil
}

func ApplyCoupon(c *gin.Context, input models.ApplyCouponPayload, clientIP string) (*models.RecalculateResult, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	couponCode := input.Coupon
	if couponCode == "" {
		return nil, fmt.Errorf("mã giảm giá không được để trống")
	}

	// 1. Tìm CartID của người dùng hiện tại (User đã đăng nhập hoặc Guest qua IP)
	var cartID int64
	if customerID, exists := c.Get("customerID"); exists && customerID != nil {
		err = db.QueryRowContext(c.Request.Context(), "SELECT id FROM carts WHERE customer_id = ? AND deleted_at IS NULL LIMIT 1", customerID).Scan(&cartID)
	} else {
		err = db.QueryRowContext(c.Request.Context(), "SELECT id FROM carts WHERE ip_address = ? AND customer_id IS NULL AND deleted_at IS NULL LIMIT 1", clientIP).Scan(&cartID)
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("không tìm thấy giỏ hàng")
		}
		return nil, err
	}

	// 2. Tìm mã coupon trong database
	var couponID int64
	err = db.QueryRowContext(c.Request.Context(), "SELECT id FROM coupons WHERE code = ? AND deleted_at IS NULL LIMIT 1", couponCode).Scan(&couponID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("mã giảm giá không hợp lệ hoặc đã hết hạn")
		}
		return nil, err
	}

	// 3. Cập nhật coupon_id vào giỏ hàng
	_, err = db.ExecContext(c.Request.Context(), "UPDATE carts SET coupon_id = ?, updated_at = NOW() WHERE id = ?", couponID, cartID)
	if err != nil {
		return nil, fmt.Errorf("không thể áp dụng mã giảm giá: %v", err)
	}

	// 4. Lấy lại dữ liệu Cart mới nhất
	cart, err := GetCartById(c, cartID)
	if err != nil {
		return nil, err
	}

	// 5. Tính toán lại giỏ hàng và trả về kết quả (đã bao gồm package services / shippingMethods)
	return CartRecalculate(c, cart)
}
