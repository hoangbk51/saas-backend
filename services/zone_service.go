package services

import (
	"net/http"
	"strconv"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

// 1. Get List Zones
func GetZonesService(c *gin.Context) (interface{}, int, error) {
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
	search := c.Query("search")

	var totalCount int
	countQuery := `SELECT COUNT(*) FROM zones WHERE deleted_at IS NULL`
	argsCount := []interface{}{}

	if search != "" {
		countQuery += ` AND (name LIKE ? OR description LIKE ?)`
		searchTerm := "%" + search + "%"
		argsCount = append(argsCount, searchTerm, searchTerm)
	}

	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery, argsCount...).Scan(&totalCount)
	if err != nil {
		return gin.H{"error": "Lỗi đếm số lượng zones: " + err.Error()}, http.StatusInternalServerError, err
	}

	zones := make([]map[string]interface{}, 0)
	if totalCount == 0 {
		return utils.BuildLaravelPagination(c, zones, totalCount, page, limit), http.StatusOK, nil
	}

	query := `
		SELECT id, name, description, created_at, updated_at
		FROM zones
		WHERE deleted_at IS NULL
	`
	args := []interface{}{}

	if search != "" {
		query += ` AND (name LIKE ? OR description LIKE ?)`
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn zones: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := utils.ScanRowToMap(rows)
		if err == nil {
			zones = append(zones, item)
		}
	}

	return utils.BuildLaravelPagination(c, zones, totalCount, page, limit), http.StatusOK, nil
}

// 2. Get Zone Detail By ID
func GetZoneByIDService(c *gin.Context, id int) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	queryZone := `SELECT id, name, description, created_at, updated_at FROM zones WHERE id = ? AND deleted_at IS NULL LIMIT 1`
	rows, err := tenantDB.QueryContext(c.Request.Context(), queryZone, id)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn zone: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Không tìm thấy zone"}, http.StatusNotFound, nil
	}

	zoneData, err := utils.ScanRowToMap(rows)
	if err != nil {
		return gin.H{"error": "Lỗi scan zone data"}, http.StatusInternalServerError, err
	}

	queryDetails := `
		SELECT id, country_id, state_id, created_at, updated_at 
		FROM zone_detail 
		WHERE zone_id = ? AND deleted_at IS NULL
	`
	detailRows, err := tenantDB.QueryContext(c.Request.Context(), queryDetails, id)
	if err == nil {
		defer detailRows.Close()
		details := make([]map[string]interface{}, 0)
		for detailRows.Next() {
			dItem, err := utils.ScanRowToMap(detailRows)
			if err == nil {
				details = append(details, dItem)
			}
		}
		zoneData["details"] = details
	}

	return gin.H{"status": "success", "data": zoneData}, http.StatusOK, nil
}

// 3. Create Zone
func CreateZoneService(c *gin.Context, req models.CreateZoneReq) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	tx, err := tenantDB.BeginTxx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	insertZoneQuery := `INSERT INTO zones (name, description, created_at, updated_at) VALUES (?, ?, NOW(), NOW())`
	res, err := tx.ExecContext(c.Request.Context(), insertZoneQuery, req.Name, req.Description)
	if err != nil {
		return gin.H{"error": "Lỗi tạo zone: " + err.Error()}, http.StatusInternalServerError, err
	}

	zoneID, _ := res.LastInsertId()

	if len(req.Details) > 0 {
		insertDetailQuery := `INSERT INTO zone_detail (country_id, state_id, zone_id, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())`
		for _, d := range req.Details {
			_, err := tx.ExecContext(c.Request.Context(), insertDetailQuery, d.CountryID, d.StateID, zoneID)
			if err != nil {
				return gin.H{"error": "Lỗi tạo zone detail: " + err.Error()}, http.StatusInternalServerError, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi commit transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"status": "success", "message": "Tạo zone thành công", "id": zoneID}, http.StatusCreated, nil
}

// 4. Update Zone
func UpdateZoneService(c *gin.Context, id int, req models.UpdateZoneReq) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	tx, err := tenantDB.BeginTxx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	updateZoneQuery := `UPDATE zones SET name = ?, description = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL`
	res, err := tx.ExecContext(c.Request.Context(), updateZoneQuery, req.Name, req.Description, id)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật zone: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Không tìm thấy zone hoặc không có thay đổi"}, http.StatusNotFound, nil
	}

	_, err = tx.ExecContext(c.Request.Context(), `UPDATE zone_detail SET deleted_at = NOW() WHERE zone_id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return gin.H{"error": "Lỗi xóa zone detail cũ: " + err.Error()}, http.StatusInternalServerError, err
	}

	if len(req.Details) > 0 {
		insertDetailQuery := `INSERT INTO zone_detail (country_id, state_id, zone_id, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())`
		for _, d := range req.Details {
			_, err := tx.ExecContext(c.Request.Context(), insertDetailQuery, d.CountryID, d.StateID, id)
			if err != nil {
				return gin.H{"error": "Lỗi thêm zone detail mới: " + err.Error()}, http.StatusInternalServerError, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi commit transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"status": "success", "message": "Cập nhật zone thành công"}, http.StatusOK, nil
}

// 5. Delete Zone
func DeleteZoneService(c *gin.Context, id int) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	tx, err := tenantDB.BeginTxx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(c.Request.Context(), `UPDATE zones SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return gin.H{"error": "Lỗi xóa zone: " + err.Error()}, http.StatusInternalServerError, err
	}

	_, err = tx.ExecContext(c.Request.Context(), `UPDATE zone_detail SET deleted_at = NOW() WHERE zone_id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return gin.H{"error": "Lỗi xóa zone detail: " + err.Error()}, http.StatusInternalServerError, err
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi commit transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"status": "success", "message": "Xóa zone thành công"}, http.StatusOK, nil
}
