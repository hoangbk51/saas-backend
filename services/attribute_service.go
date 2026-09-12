package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

// 2. GET ATTRIBUTE BY ID
func GetAttributeByIDService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")
	query := `
		SELECT id, name, attribute_type_id, ` + "`order`" + `, filterable, created_at, updated_at 
		FROM attributes 
		WHERE id = ? AND deleted_at IS NULL
	`
	rows, err := tenantDB.QueryContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Thuộc tính không tồn tại"}, http.StatusNotFound, nil
	}

	attr, err := utils.ScanRowToMap(rows)
	if err != nil {
		return gin.H{"error": "Lỗi parse dữ liệu"}, http.StatusInternalServerError, err
	}

	if nameStr, ok := attr["name"].(string); ok && nameStr != "" {
		var nameMap map[string]string
		if err := json.Unmarshal([]byte(nameStr), &nameMap); err == nil {
			attr["name"] = nameMap
		}
	}

	// Lấy danh sách Values
	valRows, err := tenantDB.QueryContext(c.Request.Context(), `
		SELECT id, name, color, sort_order 
		FROM attribute_values 
		WHERE attribute_id = ? AND deleted_at IS NULL 
		ORDER BY sort_order ASC, id ASC
	`, id)

	valuesList := make([]map[string]interface{}, 0)
	if err == nil {
		for valRows.Next() {
			valItem, vErr := utils.ScanRowToMap(valRows)
			if vErr == nil {
				if vNameStr, ok := valItem["name"].(string); ok && vNameStr != "" {
					var vNameMap map[string]string
					if err := json.Unmarshal([]byte(vNameStr), &vNameMap); err == nil {
						valItem["name"] = vNameMap
					}
				}
				valuesList = append(valuesList, valItem)
			}
		}
		valRows.Close()
	}
	attr["values"] = valuesList

	return gin.H{"data": attr}, http.StatusOK, nil
}

// 1. GET LIST ATTRIBUTES
func GetAttributesService(c *gin.Context) (interface{}, int, error) {
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
	countQuery := `SELECT COUNT(*) FROM attributes WHERE deleted_at IS NULL`
	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery).Scan(&totalCount)
	if err != nil {
		return gin.H{"error": "Lỗi đếm số lượng attributes: " + err.Error()}, http.StatusInternalServerError, err
	}

	attributes := make([]map[string]interface{}, 0)
	if totalCount == 0 {
		return utils.BuildLaravelPagination(c, attributes, totalCount, page, limit), http.StatusOK, nil
	}

	query := `
		SELECT id, name, attribute_type_id, ` + "`order`" + `, filterable, created_at, updated_at 
		FROM attributes 
		WHERE deleted_at IS NULL 
		ORDER BY ` + "`order`" + ` ASC, id DESC 
		LIMIT ? OFFSET ?
	`

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, limit, offset)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn attributes: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := utils.ScanRowToMap(rows)
		if err != nil {
			continue
		}

		if nameStr, ok := item["name"].(string); ok && nameStr != "" {
			var nameMap map[string]string
			if err := json.Unmarshal([]byte(nameStr), &nameMap); err == nil {
				item["name"] = nameMap
			}
		}

		attrID := item["id"]
		valRows, err := tenantDB.QueryContext(c.Request.Context(), `
			SELECT id, name, color, sort_order 
			FROM attribute_values 
			WHERE attribute_id = ? AND deleted_at IS NULL 
			ORDER BY sort_order ASC, id ASC
		`, attrID)

		valuesList := make([]map[string]interface{}, 0)
		if err == nil {
			for valRows.Next() {
				valItem, vErr := utils.ScanRowToMap(valRows)
				if vErr == nil {
					if vNameStr, ok := valItem["name"].(string); ok && vNameStr != "" {
						var vNameMap map[string]string
						if err := json.Unmarshal([]byte(vNameStr), &vNameMap); err == nil {
							valItem["name"] = vNameMap
						}
					}
					valuesList = append(valuesList, valItem)
				}
			}
			valRows.Close()
		}

		item["values"] = valuesList
		attributes = append(attributes, item)
	}

	return utils.BuildLaravelPagination(c, attributes, totalCount, page, limit), http.StatusOK, nil
}

// 2. CREATE ATTRIBUTE
func CreateAttributeService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.AttributePayload
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	nameJSON, _ := json.Marshal(input.Name)

	// Xử lý giá trị filterable (*bool): Mặc định là 1 (true) trừ khi client truyền false
	filterableVal := 1
	if input.Filterable != nil && !*input.Filterable {
		filterableVal = 0
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Không thể bắt đầu Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	attrQuery := `
		INSERT INTO attributes (name, attribute_type_id, ` + "`order`" + `, filterable, created_at, updated_at) 
		VALUES (?, ?, ?, ?, NOW(), NOW())
	`
	res, err := tx.ExecContext(c.Request.Context(), attrQuery, string(nameJSON), input.AttributeTypeID, input.Order, filterableVal)
	if err != nil {
		return gin.H{"error": "Lỗi thêm Attribute: " + err.Error()}, http.StatusInternalServerError, err
	}

	attrID, _ := res.LastInsertId()

	if len(input.Values) > 0 {
		valQuery := `
			INSERT INTO attribute_values (name, color, attribute_id, sort_order, created_at, updated_at) 
			VALUES (?, ?, ?, ?, NOW(), NOW())
		`
		for _, val := range input.Values {
			vNameJSON, _ := json.Marshal(val.Name)
			_, err := tx.ExecContext(c.Request.Context(), valQuery, string(vNameJSON), val.Color, attrID, val.SortOrder)
			if err != nil {
				return gin.H{"error": "Lỗi thêm Attribute Value: " + err.Error()}, http.StatusInternalServerError, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi Commit Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Tạo thuộc tính thành công", "id": attrID}, http.StatusCreated, nil
}

// 3. UPDATE ATTRIBUTE
// UPDATE ATTRIBUTE (Kèm xóa theo danh sách delete_attribute_value_ids)
// UPDATE ATTRIBUTE
func UpdateAttributeService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	var input models.AttributePayload
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	nameJSON, _ := json.Marshal(input.Name)

	filterableVal := 1
	if input.Filterable != nil && !*input.Filterable {
		filterableVal = 0
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// 1. Cập nhật bảng cha (attributes)
	attrQuery := `
		UPDATE attributes 
		SET name = ?, attribute_type_id = ?, ` + "`order`" + ` = ?, filterable = ?, updated_at = NOW() 
		WHERE id = ? AND deleted_at IS NULL
	`
	res, err := tx.ExecContext(c.Request.Context(), attrQuery, string(nameJSON), input.AttributeTypeID, input.Order, filterableVal, id)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật Attribute: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Thuộc tính không tồn tại hoặc đã bị xóa"}, http.StatusNotFound, nil
	}

	// 2. Soft delete danh sách delete_attribute_value_ids bằng 1 Query duy nhất (Dùng deleteValQuery)
	if len(input.DeleteAttributeValueIDs) > 0 {
		// Tạo chuỗi placeholders (?, ?, ?) tương ứng với số lượng ID
		placeholders := make([]string, len(input.DeleteAttributeValueIDs))
		args := make([]interface{}, 0, len(input.DeleteAttributeValueIDs)+1)

		args = append(args, id) // Tham số cho attribute_id
		for i, valID := range input.DeleteAttributeValueIDs {
			placeholders[i] = "?"
			args = append(args, valID)
		}

		deleteValQuery := fmt.Sprintf(`
			UPDATE attribute_values 
			SET deleted_at = NOW() 
			WHERE attribute_id = ? AND id IN (%s) AND deleted_at IS NULL
		`, strings.Join(placeholders, ","))

		_, err := tx.ExecContext(c.Request.Context(), deleteValQuery, args...)
		if err != nil {
			return gin.H{"error": "Lỗi xóa Attribute Values: " + err.Error()}, http.StatusInternalServerError, err
		}
	}

	// 3. Upsert danh sách Values (Thêm mới nếu id = 0, Cập nhật nếu có id)
	if len(input.Values) > 0 {
		insertValQuery := `
			INSERT INTO attribute_values (name, color, attribute_id, sort_order, created_at, updated_at) 
			VALUES (?, ?, ?, ?, NOW(), NOW())
		`
		updateValQuery := `
			UPDATE attribute_values 
			SET name = ?, color = ?, sort_order = ?, updated_at = NOW() 
			WHERE id = ? AND attribute_id = ? AND deleted_at IS NULL
		`

		for _, val := range input.Values {
			vNameJSON, _ := json.Marshal(val.Name)

			if val.ID > 0 {
				// Đã có ID -> Cập nhật
				_, err := tx.ExecContext(c.Request.Context(), updateValQuery, string(vNameJSON), val.Color, val.SortOrder, val.ID, id)
				if err != nil {
					return gin.H{"error": "Lỗi cập nhật Attribute Value: " + err.Error()}, http.StatusInternalServerError, err
				}
			} else {
				// Chưa có ID -> Thêm mới
				_, err := tx.ExecContext(c.Request.Context(), insertValQuery, string(vNameJSON), val.Color, id, val.SortOrder)
				if err != nil {
					return gin.H{"error": "Lỗi thêm mới Attribute Value: " + err.Error()}, http.StatusInternalServerError, err
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi Commit Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Cập nhật thuộc tính thành công"}, http.StatusOK, nil
}

// 5. DELETE ATTRIBUTE (Soft Delete cả Attribute và AttributeValues)
func DeleteAttributeService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Lỗi khởi tạo Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// Soft Delete Attribute
	res, err := tx.ExecContext(c.Request.Context(), `UPDATE attributes SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return gin.H{"error": "Lỗi xóa Attribute: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Thuộc tính không tồn tại hoặc đã bị xóa"}, http.StatusNotFound, nil
	}

	// Soft Delete các values liên quan
	_, err = tx.ExecContext(c.Request.Context(), `UPDATE attribute_values SET deleted_at = NOW() WHERE attribute_id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return gin.H{"error": "Lỗi xóa Attribute Values: " + err.Error()}, http.StatusInternalServerError, err
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi Commit Transaction"}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Xóa thuộc tính thành công"}, http.StatusOK, nil
}
