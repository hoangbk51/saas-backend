package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const timeLayout = "2006-01-02 15:04:05"

func GetAllCoupons(c *gin.Context) ([]models.Coupon, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id,code, name FROM coupons ORDER BY name ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var coupons []models.Coupon
	for rows.Next() {
		var c models.Coupon
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		coupons = append(coupons, c)
	}
	return coupons, nil
}

func GetCouponByIds(c *gin.Context) ([]models.Coupon, error) {
	ids := c.QueryArray("ids[]")

	// Chuyển []string sang []interface{} để truyền vào hàm Query SQL
	couponIds := make([]interface{}, len(ids))
	for i, v := range ids {
		couponIds[i] = v
	}

	// Tạo placeholders: ?,?,?
	placeholders := make([]string, len(couponIds))
	for i := range couponIds {
		placeholders[i] = "?"
	}

	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
       SELECT id,code, name FROM coupons as c
        WHERE c.id IN (%s)  ORDER BY name ASC`, strings.Join(placeholders, ","))
	utils.LogSQL(query, couponIds...)
	rows, err := db.Query(query, couponIds...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var coupons []models.Coupon
	for rows.Next() {
		var c models.Coupon
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		coupons = append(coupons, c)
	}
	return coupons, nil
}

// 1. GET LIST COUPONS (Có phân trang & lọc soft delete)
func GetCouponsService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	var totalCount int
	countQuery := `SELECT COUNT(*) FROM coupons WHERE deleted_at IS NULL`
	utils.LogSQL(countQuery)

	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery).Scan(&totalCount)
	if err != nil {
		return gin.H{"error": "Lỗi đếm danh sách coupon: " + err.Error()}, http.StatusInternalServerError, err
	}

	coupons := make([]map[string]interface{}, 0)
	if totalCount == 0 {
		return utils.BuildLaravelPagination(c, coupons, totalCount, page, limit), http.StatusOK, nil
	}

	query := `
		SELECT id, name, code, description, value, min_order_amount, type, 
		       quantity, quantity_per_customer, starting_time, ending_time, active, 
		       created_at, updated_at 
		FROM coupons 
		WHERE deleted_at IS NULL 
		ORDER BY id DESC 
		LIMIT ? OFFSET ?
	`
	utils.LogSQL(query, limit, offset)

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, limit, offset)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn coupons: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := utils.ScanRowToMap(rows)
		if err != nil {
			continue
		}

		// Decode name & description JSON sang Object
		if nameStr, ok := item["name"].(string); ok && nameStr != "" {
			var nameMap map[string]string
			if err := json.Unmarshal([]byte(nameStr), &nameMap); err == nil {
				item["name"] = nameMap
			}
		}
		if descStr, ok := item["description"].(string); ok && descStr != "" {
			var descMap map[string]string
			if err := json.Unmarshal([]byte(descStr), &descMap); err == nil {
				item["description"] = descMap
			}
		}

		// Format DateTime
		formatTimeField(item, "starting_time")
		formatTimeField(item, "ending_time")
		formatTimeField(item, "created_at")
		formatTimeField(item, "updated_at")

		coupons = append(coupons, item)
	}

	return utils.BuildLaravelPagination(c, coupons, totalCount, page, limit), http.StatusOK, nil
}

// 2. GET COUPON BY ID
func GetCouponByIDService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")
	query := `
		SELECT id, name, code, description, value, min_order_amount, type, 
		       quantity, quantity_per_customer, starting_time, ending_time, active, 
		       created_at, updated_at 
		FROM coupons 
		WHERE id = ? AND deleted_at IS NULL
	`
	rows, err := tenantDB.QueryContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Coupon không tồn tại"}, http.StatusNotFound, nil
	}

	coupon, err := utils.ScanRowToMap(rows)
	if err != nil {
		return gin.H{"error": "Lỗi parse dữ liệu"}, http.StatusInternalServerError, err
	}

	// Unmarshal name & description
	if nameStr, ok := coupon["name"].(string); ok && nameStr != "" {
		var nameMap map[string]string
		if err := json.Unmarshal([]byte(nameStr), &nameMap); err == nil {
			coupon["name"] = nameMap
		}
	}
	if descStr, ok := coupon["description"].(string); ok && descStr != "" {
		var descMap map[string]string
		if err := json.Unmarshal([]byte(descStr), &descMap); err == nil {
			coupon["description"] = descMap
		}
	}

	formatTimeField(coupon, "starting_time")
	formatTimeField(coupon, "ending_time")
	formatTimeField(coupon, "created_at")
	formatTimeField(coupon, "updated_at")

	return gin.H{"data": coupon}, http.StatusOK, nil
}

// Helper chèn mảng ID vào bảng Pivot
func insertCouponRelations(ctx context.Context, tx *sql.Tx, couponID int64, productIDs []int64, categoryIDs []int64) error {
	// 1. Insert Product IDs
	if len(productIDs) > 0 {
		queryProd := `INSERT INTO coupon_products (coupon_id, product_id) VALUES `
		vals := []interface{}{}
		for i, pID := range productIDs {
			if i > 0 {
				queryProd += ", "
			}
			queryProd += "(?, ?)"
			vals = append(vals, couponID, pID)
		}
		if _, err := tx.ExecContext(ctx, queryProd, vals...); err != nil {
			return err
		}
	}

	// 2. Insert Category IDs
	if len(categoryIDs) > 0 {
		queryCat := `INSERT INTO coupon_categories (coupon_id, category_id) VALUES `
		vals := []interface{}{}
		for i, cID := range categoryIDs {
			if i > 0 {
				queryCat += ", "
			}
			queryCat += "(?, ?)"
			vals = append(vals, couponID, cID)
		}
		if _, err := tx.ExecContext(ctx, queryCat, vals...); err != nil {
			return err
		}
	}

	return nil
}

// 3. CREATE COUPON
func CreateCouponService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.Coupon
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	nameJSON, _ := json.Marshal(input.Name)
	descJSON, _ := json.Marshal(input.Description)

	activeVal := 1
	if input.Active != nil && !*input.Active {
		activeVal = 0
	}

	if input.Type == "" {
		input.Type = "amount"
	}

	var startTimePtr, endTimePtr *time.Time
	if input.StartingTime != nil && *input.StartingTime != "" {
		if t, err := time.Parse(timeLayout, *input.StartingTime); err == nil {
			startTimePtr = &t
		} else {
			return gin.H{"error": "Định dạng starting_time không hợp lệ (YYYY-MM-DD HH:mm:ss)"}, http.StatusBadRequest, nil
		}
	}

	if input.EndingTime != nil && *input.EndingTime != "" {
		if t, err := time.Parse(timeLayout, *input.EndingTime); err == nil {
			endTimePtr = &t
		} else {
			return gin.H{"error": "Định dạng ending_time không hợp lệ (YYYY-MM-DD HH:mm:ss)"}, http.StatusBadRequest, nil
		}
	}

	// Sử dụng Transaction
	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	query := `
		INSERT INTO coupons (
			name, code, description, value, min_order_amount, type, 
			quantity, quantity_per_customer, starting_time, ending_time, active, 
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	res, err := tx.ExecContext(
		c.Request.Context(), query,
		string(nameJSON), input.Code, string(descJSON), input.Value, input.MinOrderAmount, input.Type,
		input.Quantity, input.QuantityPerCustomer, startTimePtr, endTimePtr, activeVal,
	)
	if err != nil {
		return gin.H{"error": "Lỗi tạo coupon: " + err.Error()}, http.StatusInternalServerError, err
	}

	couponID, _ := res.LastInsertId()

	// Lưu quan hệ Products / Categories
	if err := insertCouponRelations(c.Request.Context(), tx, couponID, input.ProductIDs, input.CategoryIDs); err != nil {
		return gin.H{"error": "Lỗi gán sản phẩm/danh mục cho coupon: " + err.Error()}, http.StatusInternalServerError, err
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi lưu dữ liệu: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Tạo coupon thành công", "id": couponID}, http.StatusCreated, nil
}

// 4. UPDATE COUPON
func UpdateCouponService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	var input models.Coupon
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	nameJSON, _ := json.Marshal(input.Name)
	descJSON, _ := json.Marshal(input.Description)

	activeVal := 1
	if input.Active != nil && !*input.Active {
		activeVal = 0
	}

	if input.Type == "" {
		input.Type = "amount"
	}

	var startTimePtr, endTimePtr *time.Time
	if input.StartingTime != nil && *input.StartingTime != "" {
		if t, err := time.Parse(timeLayout, *input.StartingTime); err == nil {
			startTimePtr = &t
		} else {
			return gin.H{"error": "Định dạng starting_time không hợp lệ (YYYY-MM-DD HH:mm:ss)"}, http.StatusBadRequest, nil
		}
	}

	if input.EndingTime != nil && *input.EndingTime != "" {
		if t, err := time.Parse(timeLayout, *input.EndingTime); err == nil {
			endTimePtr = &t
		} else {
			return gin.H{"error": "Định dạng ending_time không hợp lệ (YYYY-MM-DD HH:mm:ss)"}, http.StatusBadRequest, nil
		}
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	query := `
		UPDATE coupons 
		SET name = ?, code = ?, description = ?, value = ?, min_order_amount = ?, type = ?, 
			quantity = ?, quantity_per_customer = ?, starting_time = ?, ending_time = ?, active = ?, 
			updated_at = NOW() 
		WHERE id = ? AND deleted_at IS NULL
	`

	res, err := tx.ExecContext(
		c.Request.Context(), query,
		string(nameJSON), input.Code, string(descJSON), input.Value, input.MinOrderAmount, input.Type,
		input.Quantity, input.QuantityPerCustomer, startTimePtr, endTimePtr, activeVal,
		id,
	)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật coupon: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Coupon không tồn tại hoặc đã bị xóa"}, http.StatusNotFound, nil
	}

	couponID, _ := strconv.ParseInt(id, 10, 64)

	// Xóa các liên kết cũ trong bảng pivot
	if _, err := tx.ExecContext(c.Request.Context(), "DELETE FROM coupon_products WHERE coupon_id = ?", couponID); err != nil {
		return gin.H{"error": "Lỗi làm sạch dữ liệu product cũ: " + err.Error()}, http.StatusInternalServerError, err
	}
	if _, err := tx.ExecContext(c.Request.Context(), "DELETE FROM coupon_categories WHERE coupon_id = ?", couponID); err != nil {
		return gin.H{"error": "Lỗi làm sạch dữ liệu category cũ: " + err.Error()}, http.StatusInternalServerError, err
	}

	// Gán lại quan hệ mới
	if err := insertCouponRelations(c.Request.Context(), tx, couponID, input.ProductIDs, input.CategoryIDs); err != nil {
		return gin.H{"error": "Lỗi cập nhật sản phẩm/danh mục cho coupon: " + err.Error()}, http.StatusInternalServerError, err
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi lưu dữ liệu: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Cập nhật coupon thành công"}, http.StatusOK, nil
}

// 5. DELETE COUPON (Soft Delete)
func DeleteCouponService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	query := `UPDATE coupons SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	res, err := tenantDB.ExecContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": "Không thể xóa coupon: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Coupon không tồn tại hoặc đã bị xóa trước đó"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa coupon thành công"}, http.StatusOK, nil
}

// Hàm bổ trợ format DateTime
func formatTimeField(item map[string]interface{}, key string) {
	if t, ok := item[key].(time.Time); ok && !t.IsZero() {
		item[key] = t.Format(timeLayout)
	} else {
		item[key] = nil
	}
}
