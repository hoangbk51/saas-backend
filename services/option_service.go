package services

import (
	"encoding/json"
	"go-saas/models"
	"go-saas/utils"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GetOptionsService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	// 1. Phân trang
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	// 2. Count tổng số option
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM options`
	utils.LogSQL(countQuery)

	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery).Scan(&totalCount)
	if err != nil {
		return gin.H{"error": "Lỗi đếm số option: " + err.Error()}, http.StatusInternalServerError, err
	}

	// Khởi tạo slice rỗng bằng make để tránh bị null trong JSON
	options := make([]map[string]interface{}, 0)

	if totalCount == 0 {
		response := utils.BuildLaravelPagination(c, options, totalCount, page, limit)
		return response, http.StatusOK, nil
	}

	// 3. Query danh sách options
	query := `
		SELECT id, name, type, sort_order, created_at, updated_at 
		FROM options 
		ORDER BY sort_order ASC, id DESC 
		LIMIT ? OFFSET ?
	`
	utils.LogSQL(query, limit, offset)

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, limit, offset)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn options: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	const timeLayout = "2006-01-02 15:04:05"

	for rows.Next() {
		item, err := utils.ScanRowToMap(rows)
		if err != nil {
			continue
		}

		// Decode name từ JSON string trong DB sang map/object đa ngôn ngữ
		if nameStr, ok := item["name"].(string); ok && nameStr != "" {
			var nameMap map[string]string
			if err := json.Unmarshal([]byte(nameStr), &nameMap); err == nil {
				item["name"] = nameMap
			}
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

		// 4. (Tùy chọn) Query kèm danh sách option_values cho từng option
		optionID := item["id"]
		valQuery := `
			SELECT id, name, image, sort_order 
			FROM option_values 
			WHERE option_id = ? 
			ORDER BY sort_order ASC, id ASC
		`
		valRows, errVal := tenantDB.QueryContext(c.Request.Context(), valQuery, optionID)
		valuesList := make([]map[string]interface{}, 0)

		if errVal == nil {
			for valRows.Next() {
				valItem, errScan := utils.ScanRowToMap(valRows)
				if errScan != nil {
					continue
				}
				// Decode name của option value
				if valNameStr, ok := valItem["name"].(string); ok && valNameStr != "" {
					var valNameMap map[string]string
					if err := json.Unmarshal([]byte(valNameStr), &valNameMap); err == nil {
						valItem["name"] = valNameMap
					}
				}
				valuesList = append(valuesList, valItem)
			}
			valRows.Close()
		}

		item["values"] = valuesList

		options = append(options, item)
	}

	// 5. Đóng gói Laravel Pagination
	response := utils.BuildLaravelPagination(c, options, totalCount, page, limit)
	return response, http.StatusOK, nil
}

// 1. GET DETAIL BY ID (Lấy Option kèm danh sách Values)
func GetOptionByIDService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	// Query Option
	queryOption := `SELECT id, name, type, sort_order, created_at, updated_at FROM options WHERE id = ?`
	rows, err := tenantDB.QueryContext(c.Request.Context(), queryOption, id)
	if err != nil {
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Option không tồn tại"}, http.StatusNotFound, nil
	}

	option, err := utils.ScanRowToMap(rows)
	if err != nil {
		return gin.H{"error": "Lỗi parse dữ liệu"}, http.StatusInternalServerError, err
	}

	// Decode chuỗi JSON name của Option
	if nameStr, ok := option["name"].(string); ok && nameStr != "" {
		var nameMap map[string]string
		if err := json.Unmarshal([]byte(nameStr), &nameMap); err == nil {
			option["name"] = nameMap
		}
	}

	// Query lấy danh sách Values thuộc Option này
	queryValues := `
		SELECT id, name, image, option_id, sort_order, created_at, updated_at 
		FROM option_values 
		WHERE option_id = ? 
		ORDER BY sort_order ASC, id ASC
	`
	valRows, err := tenantDB.QueryContext(c.Request.Context(), queryValues, id)
	values := make([]map[string]interface{}, 0)

	if err == nil {
		defer valRows.Close()
		for valRows.Next() {
			valItem, err := utils.ScanRowToMap(valRows)
			if err != nil {
				continue
			}
			// Decode name của Option Value
			if nameStr, ok := valItem["name"].(string); ok && nameStr != "" {
				var nameMap map[string]string
				if err := json.Unmarshal([]byte(nameStr), &nameMap); err == nil {
					valItem["name"] = nameMap
				}
			}
			values = append(values, valItem)
		}
	}

	option["values"] = values

	return gin.H{"data": option}, http.StatusOK, nil
}

// 2. CREATE OPTION (Lưu Option + Lưu toàn bộ Values)
func CreateOptionService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.OptionPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	nameJSON, err := json.Marshal(input.Name)
	if err != nil {
		return gin.H{"error": "Lỗi format dữ liệu Name"}, http.StatusBadRequest, err
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Không thể khởi tạo Transaction"}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	queryOption := `
		INSERT INTO options (name,  type, sort_order, created_at, updated_at) 
		VALUES (?, ?, ?,  NOW(), NOW())
	`
	res, err := tx.ExecContext(c.Request.Context(), queryOption, string(nameJSON), input.Type, input.SortOrder)
	if err != nil {
		log.Printf("Create option error: %v", err)
		return gin.H{"error": "Lỗi tạo option: " + err.Error()}, http.StatusInternalServerError, err
	}

	optionID, _ := res.LastInsertId()

	// 2. Insert danh sách Option Values con (input.Values giờ đây đã nhận đủ 2 item từ payload)
	if len(input.Values) > 0 {
		queryVal := `
			INSERT INTO option_values (option_id, name, image, sort_order, created_at, updated_at) 
			VALUES (?, ?, ?, ?, NOW(), NOW())
		`
		for _, val := range input.Values {
			valNameJSON, err := json.Marshal(val.Name)
			if err != nil {
				log.Printf("Marshal val name error: %v", err)
				continue
			}

			_, err = tx.ExecContext(c.Request.Context(), queryVal, optionID, string(valNameJSON), val.Image, val.SortOrder)
			if err != nil {
				log.Printf("Create option value error: %v", err)
				return gin.H{"error": "Lỗi tạo option value: " + err.Error()}, http.StatusInternalServerError, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi commit dữ liệu"}, http.StatusInternalServerError, err
	}

	return gin.H{
		"message": "Tạo option thành công",
		"id":      optionID,
	}, http.StatusCreated, nil
}

// UPDATE OPTION (Cập nhật Option + Insert/Update Values + Delete theo danh sách ID truyền lên)
func UpdateOptionService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	var input models.OptionPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	nameJSON, err := json.Marshal(input.Name)
	if err != nil {
		return gin.H{"error": "Lỗi format dữ liệu Name"}, http.StatusBadRequest, err
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Không thể khởi tạo Transaction"}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// 1. Cập nhật thông tin Option cha
	queryOption := `
		UPDATE options 
		SET name = ?,  type = ?,  sort_order = ?, updated_at = NOW() 
		WHERE id = ?
	`
	_, err = tx.ExecContext(c.Request.Context(), queryOption, string(nameJSON), input.Type, input.SortOrder, id)
	if err != nil {
		log.Printf("Update option error: %v", err)
		return gin.H{"error": "Lỗi cập nhật option: " + err.Error()}, http.StatusInternalServerError, err
	}

	// 2. Vòng lặp Xử lý Thêm mới (INSERT) hoặc Cập nhật (UPDATE) cho từng Option Value
	for _, val := range input.Values {
		valNameJSON, _ := json.Marshal(val.Name)

		if val.ID > 0 {
			// Cập nhật Value cũ
			queryUpdateVal := `
				UPDATE option_values 
				SET name = ?, image = ?, sort_order = ?, updated_at = NOW() 
				WHERE id = ? AND option_id = ?
			`
			_, err = tx.ExecContext(c.Request.Context(), queryUpdateVal, string(valNameJSON), val.Image, val.SortOrder, val.ID, id)
			if err != nil {
				return gin.H{"error": "Lỗi cập nhật option value: " + err.Error()}, http.StatusInternalServerError, err
			}
		} else {
			// Thêm mới Value
			queryInsertVal := `
				INSERT INTO option_values (option_id, name, image, sort_order, created_at, updated_at) 
				VALUES (?, ?, ?, ?, NOW(), NOW())
			`
			_, err = tx.ExecContext(c.Request.Context(), queryInsertVal, id, string(valNameJSON), val.Image, val.SortOrder)
			if err != nil {
				return gin.H{"error": "Lỗi thêm mới option value: " + err.Error()}, http.StatusInternalServerError, err
			}
		}
	}

	// 3. Xóa các Option Value có ID nằm trong danh sách delete_option_value_ids
	if len(input.DeleteOptionValueIDs) > 0 {
		// Tạo query DELETE IN dạng SQL gốc an toàn
		queryDelete := `DELETE FROM option_values WHERE option_id = ? AND id IN (`
		args := []interface{}{id}

		for i, delID := range input.DeleteOptionValueIDs {
			if i > 0 {
				queryDelete += ", "
			}
			queryDelete += "?"
			args = append(args, delID)
		}
		queryDelete += ")"

		_, err = tx.ExecContext(c.Request.Context(), queryDelete, args...)
		if err != nil {
			log.Printf("Delete option values error: %v", err)
			return gin.H{"error": "Lỗi xóa option values: " + err.Error()}, http.StatusInternalServerError, err
		}
	}

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi commit dữ liệu"}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Cập nhật option thành công"}, http.StatusOK, nil
}

// 4. DELETE OPTION (Xóa Option + tự động dọn dẹp Option Values)
func DeleteOptionService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Không thể kết nối DB"}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// 1. Xóa các option values trước
	_, _ = tx.ExecContext(c.Request.Context(), `DELETE FROM option_values WHERE option_id = ?`, id)

	// 2. Xóa option cha
	res, err := tx.ExecContext(c.Request.Context(), `DELETE FROM options WHERE id = ?`, id)
	if err != nil {
		return gin.H{"error": "Không thể xóa option này"}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Option không tồn tại"}, http.StatusNotFound, nil
	}

	_ = tx.Commit()
	return gin.H{"message": "Xóa option thành công"}, http.StatusOK, nil
}
