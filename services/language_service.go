package services

import (
	"go-saas/models"
	"go-saas/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Language struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func GetAllLanguages(c *gin.Context) ([]Language, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id,code, language as name FROM languages ORDER BY name ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var languages []Language
	for rows.Next() {
		var c Language
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		languages = append(languages, c)
	}
	return languages, nil
}

func FrontendLanguages(c *gin.Context) ([]Language, error) {
	// Service tự đi lấy DB từ context
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := "SELECT id,code, language as name FROM languages where active = 1 and is_frontend = 1 ORDER BY name ASC"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var languages []Language
	for rows.Next() {
		var c Language
		if err := rows.Scan(&c.ID, &c.Code, &c.Name); err != nil {
			continue
		}
		languages = append(languages, c)
	}
	return languages, nil
}

// List Languages (Phân trang & Lọc)
func ListLanguagesService(c *gin.Context) (utils.LaravelCollection, error) {
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

	whereClause := "WHERE deleted_at IS NULL"
	args := []interface{}{}

	if active != "" {
		whereClause += " AND active = ?"
		args = append(args, active)
	}
	if search != "" {
		whereClause += " AND (code LIKE ? OR language LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM languages " + whereClause
	if err := db.GetContext(c.Request.Context(), &total, countQuery, args...); err != nil {
		return utils.LaravelCollection{}, err
	}

	dataQuery := `
		SELECT 
			id, code, php_locale_code, language, ` + "`order`" + `, 
			rtl, active, is_backend, is_frontend, created_at, updated_at
		FROM languages ` + whereClause + ` ORDER BY ` + "`order`" + ` ASC, id DESC LIMIT ? OFFSET ?`

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

// Get Detail Language
func GetLanguageDetailService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `SELECT id, code, php_locale_code, language, ` + "`order`" + `, rtl, active, is_backend, is_frontend, created_at, updated_at FROM languages WHERE id = ? AND deleted_at IS NULL LIMIT 1`
	rows, err := db.QueryxContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Ngôn ngữ không tồn tại"}, http.StatusNotFound, nil
	}

	result := make(map[string]interface{})
	rows.MapScan(result)
	sanitizeByteMap(result)

	return result, http.StatusOK, nil
}

// Create Language
func CreateLanguageService(c *gin.Context) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.LanguageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		INSERT INTO languages (
			code, php_locale_code, language, ` + "`order`" + `, rtl, active, is_backend, is_frontend, created_at, updated_at
		) VALUES (?, ?, ?, COALESCE(?, 100), COALESCE(?, 0), COALESCE(?, 1), COALESCE(?, 0), COALESCE(?, 1), NOW(), NOW())`

	res, err := db.ExecContext(
		c.Request.Context(), query,
		input.Code, input.PhpLocaleCode, input.Language, input.Order, input.RTL, input.Active, input.IsBackend, input.IsFrontend,
	)
	if err != nil {
		return gin.H{"error": "Không thể tạo ngôn ngữ: " + err.Error()}, http.StatusInternalServerError, err
	}

	id, _ := res.LastInsertId()
	return gin.H{"message": "Tạo ngôn ngữ thành công", "id": id}, http.StatusCreated, nil
}

// Update Language
func UpdateLanguageService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.LanguageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	query := `
		UPDATE languages 
		SET code = ?, php_locale_code = ?, language = ?, ` + "`order`" + ` = COALESCE(?, ` + "`order`" + `), 
		    rtl = COALESCE(?, rtl), active = COALESCE(?, active), 
		    is_backend = COALESCE(?, is_backend), is_frontend = COALESCE(?, is_frontend), updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL`

	res, err := db.ExecContext(
		c.Request.Context(), query,
		input.Code, input.PhpLocaleCode, input.Language, input.Order, input.RTL, input.Active, input.IsBackend, input.IsFrontend, id,
	)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật ngôn ngữ: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Ngôn ngữ không tồn tại"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Cập nhật ngôn ngữ thành công"}, http.StatusOK, nil
}

// Soft Delete Language
func DeleteLanguageService(c *gin.Context, id int64) (interface{}, int, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	query := `UPDATE languages SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	res, err := db.ExecContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": "Lỗi khi xóa ngôn ngữ: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Ngôn ngữ không tồn tại hoặc đã bị xóa"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa ngôn ngữ thành công"}, http.StatusOK, nil
}
