package services

import (
	"go-saas/models"
	"go-saas/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Currency struct {
	ID     int     `json:"id"`
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Symbol string  `json:"symbol"`
	Rate   float64 `json:"exchange_rate"`
}

// GetAllCurrencies lấy toàn bộ danh sách tiền tệ
func GetAllCurrencies(c *gin.Context) ([]Currency, error) {
	db, err := utils.GetCentralDB()
	if err != nil {
		return nil, err
	}

	query := "SELECT id, iso_code as code, name, symbol, exchange_rate FROM currencies where active = 1 ORDER BY code ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Currency
	for rows.Next() {
		var cur Currency
		if err := rows.Scan(&cur.ID, &cur.Code, &cur.Name, &cur.Symbol, &cur.Rate); err != nil {
			continue
		}
		list = append(list, cur)
	}

	return list, nil
}

// List Currencies (Phân trang & Lọc)
func ListCurrenciesService(c *gin.Context) (utils.LaravelCollection, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return utils.LaravelCollection{}, err
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	search := c.Query("search")
	active := c.Query("active")

	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if active != "" {
		whereClause += " AND active = ?"
		args = append(args, active)
	}
	if search != "" {
		whereClause += " AND (iso_code LIKE ? OR name LIKE ? OR symbol LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM currencies " + whereClause
	if err := db.GetContext(c.Request.Context(), &total, countQuery, args...); err != nil {
		return utils.LaravelCollection{}, err
	}

	dataQuery := `
		SELECT 
			id, priority, iso_code, name, symbol, symbol_first, 
			COALESCE(decimal_mark, '.') as decimal_mark, 
			COALESCE(thousands_separator, ',') as thousands_separator, 
			active, exchange_rate, created_at, updated_at
		FROM currencies ` + whereClause + ` ORDER BY priority ASC, id DESC LIMIT ? OFFSET ?`

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

// Get Detail Currency
func GetCurrencyDetailService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `SELECT id, priority, iso_code, name, symbol, symbol_first, decimal_mark, thousands_separator, active, exchange_rate, created_at, updated_at FROM currencies WHERE id = ? LIMIT 1`
	rows, err := db.QueryxContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Tiền tệ không tồn tại"}, http.StatusNotFound, nil
	}

	result := make(map[string]interface{})
	rows.MapScan(result)
	sanitizeByteMap(result)

	return result, http.StatusOK, nil
}

// Create Currency
func CreateCurrencyService(c *gin.Context) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.CurrencyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		INSERT INTO currencies (
			priority, iso_code, name, symbol, symbol_first, decimal_mark, 
			thousands_separator, active, exchange_rate, created_at, updated_at
		) VALUES (COALESCE(?, 100), ?, ?, ?, COALESCE(?, 1), COALESCE(?, '.'), COALESCE(?, ','), COALESCE(?, 1), COALESCE(?, 1.00000), NOW(), NOW())`

	res, err := db.ExecContext(
		c.Request.Context(), query,
		input.Priority, input.ISOCode, input.Name, input.Symbol, input.SymbolFirst, input.DecimalMark, input.ThousandsSeparator, input.Active, input.ExchangeRate,
	)
	if err != nil {
		return gin.H{"error": "Không thể tạo loại tiền tệ: " + err.Error()}, http.StatusInternalServerError, err
	}

	id, _ := res.LastInsertId()
	return gin.H{"message": "Tạo tiền tệ thành công", "id": id}, http.StatusCreated, nil
}

// Update Currency
func UpdateCurrencyService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.CurrencyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		UPDATE currencies 
		SET priority = COALESCE(?, priority), iso_code = ?, name = ?, symbol = ?, 
		    symbol_first = COALESCE(?, symbol_first), decimal_mark = COALESCE(?, decimal_mark), 
		    thousands_separator = COALESCE(?, thousands_separator), active = COALESCE(?, active), 
		    exchange_rate = COALESCE(?, exchange_rate), updated_at = NOW()
		WHERE id = ?`

	res, err := db.ExecContext(
		c.Request.Context(), query,
		input.Priority, input.ISOCode, input.Name, input.Symbol, input.SymbolFirst, input.DecimalMark, input.ThousandsSeparator, input.Active, input.ExchangeRate, id,
	)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật tiền tệ: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Tiền tệ không tồn tại"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Cập nhật tiền tệ thành công"}, http.StatusOK, nil
}

// Hard Delete Currency
func DeleteCurrencyService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `DELETE FROM currencies WHERE id = ?`
	res, err := db.ExecContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": "Lỗi khi xóa tiền tệ: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Tiền tệ không tồn tại"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa tiền tệ thành công"}, http.StatusOK, nil
}
