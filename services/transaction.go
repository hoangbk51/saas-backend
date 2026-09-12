package services

import (
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GetTransactionByCode(c *gin.Context, refCode string) (*models.Transaction, error) {
	// 1. Lấy tenantDB (giả định utils trả về *sqlx.DB)
	// Nếu utils trả về *sql.DB, bạn có thể ép kiểu hoặc bọc nó: db := sqlx.NewDb(rawDB, "mysql")
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("tenant db context error: %w", err)
	}

	var transaction models.Transaction

	// 2. Viết câu Query
	query := `SELECT id, code,reference_id, amount, status, order_id, created_at 
              FROM transactions 
              WHERE code = ? 
              LIMIT 1`
	utils.LogSQL(query, refCode)

	err = db.QueryRow(query, refCode).Scan(
		&transaction.ID,
		&transaction.Code,
		&transaction.ReferenceID,
		&transaction.Amount,    // Cột 3: COALESCE(transaction.sale_price, 0)
		&transaction.Status,    // Cột 4: COALESCE(transaction.brand_id, 0)
		&transaction.OrderID,   // Cột 5: COALESCE(transaction.model_number, '')
		&transaction.CreatedAt, // Cột 6: COALESCE(transaction.purchase_price, 0)

	)

	return &transaction, nil
}
func GetCustomerTransactions(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	// 1. Lấy tham số Filter & Pagination từ Query Params
	customerID := c.Query("customer_id")
	if customerID == "" {
		return gin.H{"error": "customer_id là bắt buộc"}, http.StatusBadRequest, nil
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	// 2. Count tổng số bản ghi
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM transactions WHERE customer_id = ? AND deleted_at IS NULL`
	utils.LogSQL(countQuery, customerID)

	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery, customerID).Scan(&totalCount)
	if err != nil {
		log.Printf("Count transactions error: %v", err)
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}

	// Khởi tạo slice map chuẩn kiểu []map[string]interface{}
	transactions := make([]map[string]interface{}, 0)

	if totalCount == 0 {
		response := utils.BuildLaravelPagination(c, transactions, totalCount, page, limit)
		return response, http.StatusOK, nil
	}

	// 3. Query lấy danh sách
	query := `
		SELECT 
			id, order_id, reference_id, payment_method, 
			status, code, amount, type, customer_id, 
			created_at, updated_at
		FROM transactions 
		WHERE customer_id = ? AND deleted_at IS NULL
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`
	utils.LogSQL(query, customerID, limit, offset)

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, customerID, limit, offset)
	if err != nil {
		log.Printf("Select transactions error: %v", err)
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	// 4. Parse dữ liệu sang []map[string]interface{}
	const timeLayout = "2006-01-02 15:04:05"

	var statusLabels = map[int64]string{
		0: "Chưa thanh toán",
		1: "Đã hoàn thành",
		2: "Đã hủy",
	}

	for rows.Next() {
		item, err := utils.ScanRowToMap(rows)
		if err != nil {
			log.Println("Scan transaction row error:", err)
			continue
		}

		// Ép kiểu ép số amount về float64 chuẩn
		item["amount"] = utils.ParseToFloat(item["amount"])

		var statusVal int64
		if val, ok := item["status"].(int64); ok {
			statusVal = val
		} else if val, ok := item["status"].(int32); ok {
			statusVal = int64(val)
		}

		// Thêm field status_label vào response map
		if label, exists := statusLabels[statusVal]; exists {
			item["status_label"] = label
		} else {
			item["status_label"] = "Không xác định"
		}

		// Format thời gian
		if createdAt, ok := item["created_at"].(time.Time); ok && !createdAt.IsZero() {
			item["created_at"] = createdAt.Format(timeLayout)
		} else {
			item["created_at"] = nil
		}

		if updatedAt, ok := item["updated_at"].(time.Time); ok && !updatedAt.IsZero() {
			item["updated_at"] = updatedAt.Format(timeLayout)
		} else {
			item["updated_at"] = nil
		}

		transactions = append(transactions, item)
	}

	// 5. Đóng gói phân trang kiểu Laravel
	response := utils.BuildLaravelPagination(c, transactions, totalCount, page, limit)
	return response, http.StatusOK, nil
}
