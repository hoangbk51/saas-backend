package services

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"go-saas/models"
	"go-saas/tasks"
	"go-saas/utils"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func LogoutCustomer(c *gin.Context, authHeader string) error {
	db, err := utils.GetDBFromContext(c)

	// 1. Kiểm tra định dạng Header
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return fmt.Errorf("invalid authorization header")
	}
	rawToken := strings.TrimPrefix(authHeader, "Bearer ")

	// 2. Xử lý logic Sanctum (id|plainTextToken)
	parts := strings.SplitN(rawToken, "|", 2)
	tokenToHash := parts[0]
	if len(parts) == 2 {
		tokenToHash = parts[1]
	}

	// 3. Hash SHA-256 theo đúng chuẩn Laravel Sanctum
	h := sha256.New()
	h.Write([]byte(tokenToHash))
	hashedToken := hex.EncodeToString(h.Sum(nil))

	// 4. Dùng sqlx để thực thi DELETE vật lý
	query := "DELETE FROM personal_access_tokens WHERE token = ?"
	result, err := db.Exec(query, hashedToken)
	if err != nil {
		return err
	}

	// Kiểm tra xem có bản ghi nào bị xóa không (nếu = 0 tức là token sai hoặc đã xóa rồi)
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token not found or already invalidated")
	}

	return nil
}

func VerifyCustomer(c *gin.Context, token string) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil || tenantDB == nil {
		return fmt.Errorf("không thể kết nối database tenant")
	}

	// 1. Kiểm tra xem token có tồn tại & lấy thông tin Customer
	var customer struct {
		ID    uint64
		Email string
		Name  string
	}

	checkQuery := `
		SELECT id, email, name
		FROM customers 
		WHERE verification_token = ? AND deleted_at IS NULL 
		LIMIT 1
	`
	err = tenantDB.QueryRow(checkQuery, token).Scan(
		&customer.ID,
		&customer.Email,
		&customer.Name,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("Mã xác thực không hợp lệ hoặc đã hết hạn")
		}
		return err
	}

	// 2. Cập nhật trạng thái active = 1 và xóa verification_token (để tránh verify lại)
	updateQuery := `
		UPDATE customers 
		SET active = 1, 
		    verification_token = NULL, 
		    updated_at = NOW() 
		WHERE id = ?
	`
	_, err = tenantDB.Exec(updateQuery, customer.ID)
	if err != nil {
		return err
	}

	// =========================================================================
	// 3. ĐẨY ASYNQ TASK: GỬI WELCOME EMAIL SAU KHIM XÁC THỰC THÀNH CÔNG
	// =========================================================================
	if utils.AsynqClient != nil && customer.Email != "" {
		// Lấy tên cửa hàng và domain từ setting (nếu có)
		storeName := utils.GetSetting(c, "store_name", "Cửa hàng của chúng tôi")
		storeURL := utils.GetSetting(c, "store_url", "#")

		customerName := customer.Name
		if customerName == " " || customerName == "" {
			customerName = "Khách hàng"
		}

		// Data thay thế vào Template
		reps := map[string]any{
			"CustomerName": customerName,
			"StoreName":    storeName,
			"StoreURL":     storeURL,
			"CouponCode":   "WELCOME10", // Có thể lấy động từ setting hoặc để cố định
		}

		// Enqueue Task gửi email bất đồng bộ
		errEnq := tasks.EnqueueDynamicEmail(utils.AsynqClient, c.GetString("tenantId"), customer.Email, "customer_welcome", reps)
		if errEnq != nil {
			log.Printf("[Warning] Không thể enqueue Welcome Email cho customer ID %d: %v", customer.ID, errEnq)
		}
	}

	return nil
}

type CustomerDetailResponse struct {
	Profile      map[string]interface{}  `json:"profile"`
	Addresses    utils.LaravelCollection `json:"addresses"`
	RewardPoints utils.LaravelCollection `json:"reward_points"`
	IPs          utils.LaravelCollection `json:"ips"`
	Transactions utils.LaravelCollection `json:"transactions"`
	Histories    utils.LaravelCollection `json:"histories"`
	TryOnImages  utils.LaravelCollection `json:"try_on_images"`
	Orders       utils.LaravelCollection `json:"orders"`
}

func GetCustomerDetail(c *gin.Context, customerID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	ctx := c.Request.Context()

	// 1. Lấy Profile khách hàng
	profileQuery := `
		SELECT 
			id, name, COALESCE(nice_name, '') as nice_name, email, 
			COALESCE(phone, '') as phone, dob, COALESCE(sex, '') as sex, 
			COALESCE(description, '') as description, 
			last_visited_at, COALESCE(last_visited_from, '') as last_visited_from,
			COALESCE(stripe_id, '') as stripe_id, 
			active, accepts_marketing, created_at, updated_at
		FROM customers 
		WHERE id = ? AND deleted_at IS NULL 
		LIMIT 1`

	profileRows, err := db.QueryxContext(ctx, profileQuery, customerID)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn khách hàng: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer profileRows.Close()

	if !profileRows.Next() {
		return gin.H{"error": "Không tìm thấy khách hàng"}, http.StatusNotFound, nil
	}

	profile := make(map[string]interface{})
	if err := profileRows.MapScan(profile); err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}
	sanitizeByteMap(profile)

	// 2. Query Danh Sách Địa Chỉ (Addresses - Polymorphic model Customer)
	addressQuery := `
		SELECT id, type, COALESCE(address_title, '') as address_title, 
			COALESCE(address_line_1, '') as address_line_1, 
			COALESCE(address_line_2, '') as address_line_2, 
			COALESCE(city, '') as city, COALESCE(state_id, 0) as state_id, 
			COALESCE(zip_code, '') as zip_code, COALESCE(country_id, 0) as country_id, 
			COALESCE(phone, '') as phone, 
			COALESCE(latitude, 0) as latitude, COALESCE(longitude, 0) as longitude, 
			is_default, created_at 
		FROM addresses 
		WHERE addressable_id = ? AND addressable_type = 'App\\Models\\Tenant\\Customer'`

	countAddressQuery := `SELECT COUNT(*) FROM addresses WHERE addressable_id = ? AND addressable_type = 'App\\Models\\Tenant\\Customer'`
	addressesPaginated, _ := getPaginatedData(c, db, countAddressQuery, addressQuery, "addr_page", "addr_per_page", customerID)

	// 3. Query Danh Sách Điểm Thưởng (Reward Points)
	rewardQuery := `
		SELECT id, customer_id, order_id, total, status, created_at, updated_at 
		FROM reward_points 
		WHERE customer_id = ?`
	countRewardQuery := `SELECT COUNT(*) FROM reward_points WHERE customer_id = ?`
	rewardPointsPaginated, _ := getPaginatedData(c, db, countRewardQuery, rewardQuery, "reward_page", "reward_per_page", customerID)

	// 4. Query Lịch Sử IP (Customer IPs)
	ipQuery := `SELECT customer_ip_id, ip, created_at FROM customer_ips WHERE customer_id = ?`
	countIPQuery := `SELECT COUNT(*) FROM customer_ips WHERE customer_id = ?`
	ipsPaginated, _ := getPaginatedData(c, db, countIPQuery, ipQuery, "ip_page", "ip_per_page", customerID)

	// 5. Query Giao Dịch (Customer Transactions)
	txQuery := `SELECT customer_transaction_id, order_id, description, amount, created_at FROM customer_transactions WHERE customer_id = ?`
	countTxQuery := `SELECT COUNT(*) FROM customer_transactions WHERE customer_id = ?`
	txPaginated, _ := getPaginatedData(c, db, countTxQuery, txQuery, "tx_page", "tx_per_page", customerID)

	// 6. Query Ghi Chú / Lịch Sử Chăm Sóc (Customer Histories)
	historyQuery := `SELECT customer_history_id, comment, created_at FROM customer_histories WHERE customer_id = ?`
	countHistoryQuery := `SELECT COUNT(*) FROM customer_histories WHERE customer_id = ?`
	historiesPaginated, _ := getPaginatedData(c, db, countHistoryQuery, historyQuery, "history_page", "history_per_page", customerID)

	// 7. Query Ảnh Thử Đồ (Try On Requests)
	tryOnQuery := `
		SELECT id, product_id, provider_id, result_image, status, created_at 
		FROM try_on_requests 
		WHERE customer_id = ? AND result_image IS NOT NULL AND result_image != ''`
	countTryOnQuery := `SELECT COUNT(*) FROM try_on_requests WHERE customer_id = ? AND result_image IS NOT NULL AND result_image != ''`
	tryOnPaginated, _ := getPaginatedData(c, db, countTryOnQuery, tryOnQuery, "tryon_page", "tryon_per_page", customerID)

	// 8. Query Đơn Hàng (Orders)
	orderQuery := `
		SELECT id, order_number, grand_total, order_status_id, payment_status, created_at 
		FROM orders 
		WHERE customer_id = ? AND deleted_at IS NULL`
	countOrderQuery := `SELECT COUNT(*) FROM orders WHERE customer_id = ? AND deleted_at IS NULL`
	ordersPaginated, _ := getPaginatedData(c, db, countOrderQuery, orderQuery, "order_page", "order_per_page", customerID)

	response := CustomerDetailResponse{
		Profile:      profile,
		Addresses:    addressesPaginated,
		RewardPoints: rewardPointsPaginated,
		IPs:          ipsPaginated,
		Transactions: txPaginated,
		Histories:    historiesPaginated,
		TryOnImages:  tryOnPaginated,
		Orders:       ordersPaginated,
	}

	return response, http.StatusOK, nil
}

// Helper phân trang tái sử dụng cho từng tab/bảng
func getPaginatedData(
	c *gin.Context,
	db *sqlx.DB,
	countQuery string,
	dataQuery string,
	pageParam string,
	perPageParam string,
	args ...interface{},
) (utils.LaravelCollection, error) {

	page, _ := strconv.Atoi(c.DefaultQuery(pageParam, c.DefaultQuery("page", "1")))
	perPage, _ := strconv.Atoi(c.DefaultQuery(perPageParam, c.DefaultQuery("per_page", "15")))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	// 1. Tính tổng số bản ghi
	var total int
	err := db.GetContext(c.Request.Context(), &total, countQuery, args...)
	if err != nil {
		return utils.LaravelCollection{}, err
	}

	// 2. Thêm ORDER BY, LIMIT, OFFSET vào query chính
	finalQuery := dataQuery + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, perPage, offset)

	utils.LogSQL(finalQuery, queryArgs...)

	results := []map[string]interface{}{}
	rows, err := db.QueryxContext(c.Request.Context(), finalQuery, queryArgs...)
	if err != nil {
		return utils.LaravelCollection{}, err
	}
	defer rows.Close()

	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err == nil {
			sanitizeByteMap(row)
			results = append(results, row)
		}
	}

	// 3. Build response theo chuẩn Laravel Pagination
	return utils.BuildLaravelPagination(c, results, total, page, perPage), nil
}

func sanitizeByteMap(m map[string]interface{}) {
	for k, v := range m {
		if b, ok := v.([]byte); ok {
			m[k] = string(b)
		}
	}
}

func ListCustomersService(c *gin.Context) (utils.LaravelCollection, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return utils.LaravelCollection{}, err
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}

	if search != "" {
		whereClause += " AND (name LIKE ? OR email LIKE ? OR phone LIKE ?)"
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	// 1. Tính tổng số bản ghi
	var total int
	countQuery := "SELECT COUNT(*) FROM customers " + whereClause
	if err := db.GetContext(c.Request.Context(), &total, countQuery, args...); err != nil {
		return utils.LaravelCollection{}, err
	}

	// 2. Query lấy dữ liệu
	dataQuery := `
		SELECT 
			id, name, COALESCE(nice_name, '') as nice_name, email, 
			COALESCE(phone, '') as phone, dob, COALESCE(sex, '') as sex, 
			active, accepts_marketing, created_at, updated_at 
		FROM customers ` + whereClause + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	queryArgs := append(args, perPage, offset)
	utils.LogSQL(dataQuery, queryArgs...)

	results := []map[string]interface{}{}
	rows, err := db.QueryxContext(c.Request.Context(), dataQuery, queryArgs...)
	if err != nil {
		return utils.LaravelCollection{}, err
	}
	defer rows.Close()

	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err == nil {
			sanitizeByteMap(row)
			results = append(results, row)
		}
	}

	return utils.BuildLaravelPagination(c, results, total, page, perPage), nil
}

func CreateCustomerService(c *gin.Context) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.CustomerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		INSERT INTO customers (
			name, nice_name, email, password, phone, dob, sex, description, 
			active, accepts_marketing, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	res, err := db.ExecContext(
		c.Request.Context(), query,
		input.Name, input.NiceName, input.Email, input.Password, input.Phone,
		input.Dob, input.Sex, input.Description, input.Active, input.AcceptsMarketing,
	)
	if err != nil {
		return gin.H{"error": "Không thể tạo khách hàng: " + err.Error()}, http.StatusInternalServerError, err
	}

	customerID, _ := res.LastInsertId()
	return gin.H{"message": "Tạo khách hàng thành công", "id": customerID}, http.StatusCreated, nil
}

func UpdateCustomerService(c *gin.Context, customerID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.CustomerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		UPDATE customers 
		SET name = ?, nice_name = ?, email = ?, phone = ?, dob = ?, sex = ?, 
		    description = ?, active = ?, accepts_marketing = ?, updated_at = NOW() 
		WHERE id = ? AND deleted_at IS NULL`

	res, err := db.ExecContext(
		c.Request.Context(), query,
		input.Name, input.NiceName, input.Email, input.Phone, input.Dob, input.Sex,
		input.Description, input.Active, input.AcceptsMarketing, customerID,
	)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật khách hàng: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Khách hàng không tồn tại hoặc đã bị xóa"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Cập nhật thông tin khách hàng thành công"}, http.StatusOK, nil
}

func DeleteCustomerService(c *gin.Context, customerID int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `UPDATE customers SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	res, err := db.ExecContext(c.Request.Context(), query, customerID)
	if err != nil {
		return gin.H{"error": "Lỗi khi xóa khách hàng: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Khách hàng không tồn tại hoặc đã bị xóa trước đó"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa khách hàng thành công"}, http.StatusOK, nil
}
