package services

import (
	"errors"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

/*
// GetAddresses lấy danh sách địa chỉ của một đối tượng (ví dụ Customer)
func GetAddresses(c *gin.Context, ownerID int, ownerType string) (utils.LaravelCollection, error) {
	db, _ := utils.GetDBFromContext(c)
	//var list []models.Address
	results := make([]map[string]interface{}, 0)

	query := `SELECT id, type, COALESCE(address_title, ''), COALESCE(address_line_1, ''),
			  COALESCE(address_line_2, ''), COALESCE(city, ''), COALESCE(state_id, 0),
			  COALESCE(zip_code, ''), COALESCE(country_id, 0), COALESCE(phone, ''),
			  COALESCE(latitude, 0), COALESCE(longitude, 0), is_default, created_at
			  FROM addresses WHERE addressable_id = ? AND addressable_type = ?`
	utils.LogSQL(query, ownerID, ownerType)
	rows, err := db.Query(query, ownerID, ownerType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		//var a models.Address
		a := make(map[string]interface{})

		rows.Scan(&a.ID, &a.Type, &a.AddressTitle, &a.AddressLine1, &a.AddressLine2,
			&a.City, &a.StateID, &a.ZipCode, &a.CountryID, &a.Phone,
			&a.Latitude, &a.Longitude, &a.IsDefault, &a.CreatedAt)
		results = append(results, a)
	}

	response := utils.BuildLaravelPagination(c, results, total, page, perPage)

	return response, nil
}*/

func GetAddresses(c *gin.Context, ownerID int, ownerType string) (utils.LaravelCollection, error) {
	db, _ := utils.GetDBFromContext(c)

	// 1. Lấy tham số phân trang từ URL (mặc định page 1, 15 item/trang)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	// 2. Query tính tổng số bản ghi (Cần thiết cho Laravel Pagination)
	var total int
	countQuery := `SELECT COUNT(*) FROM addresses WHERE addressable_id = ? AND addressable_type = ?`
	err := db.Get(&total, countQuery, ownerID, ownerType) // Dùng sqlx.Get
	if err != nil {
		return utils.LaravelCollection{}, err
	}

	// 3. Query lấy dữ liệu có LIMIT và OFFSET
	// Lưu ý: Dùng dấu ? làm placeholder cho MySQL
	query := `SELECT id, type, COALESCE(address_title, '') as address_title, 
              COALESCE(address_line_1, '') as address_line_1, 
              COALESCE(address_line_2, '') as address_line_2, 
              COALESCE(city, '') as city, COALESCE(state_id, 0) as state_id, 
              COALESCE(zip_code, '') as zip_code, COALESCE(country_id, 0) as country_id, 
              COALESCE(phone, '') as phone, 
              COALESCE(latitude, 0) as latitude, COALESCE(longitude, 0) as longitude, 
              is_default, created_at 
              FROM addresses 
              WHERE addressable_id = ? AND addressable_type = ?
              ORDER BY created_at DESC
              LIMIT ? OFFSET ?`

	utils.LogSQL(query, ownerID, ownerType, perPage, offset)

	// Sử dụng sqlx.Select để tự động scan vào slice map hoặc struct
	// Nếu bạn muốn dùng map, sqlx hỗ trợ tốt hơn qua SliceScan hoặc dùng Struct
	results := []map[string]interface{}{}

	rows, err := db.Queryx(query, ownerID, ownerType, perPage, offset)
	if err != nil {
		return utils.LaravelCollection{}, err
	}
	defer rows.Close()

	for rows.Next() {
		row := make(map[string]interface{})
		err := rows.MapScan(row) // sqlx hỗ trợ map scan rất tiện
		if err != nil {
			return utils.LaravelCollection{}, err
		}

		// Convert byte slice (thường xảy ra với SQL driver khi scan vào interface{}) sang string nếu cần
		for k, v := range row {
			if b, ok := v.([]byte); ok {
				row[k] = string(b)
			}
		}
		results = append(results, row)
	}

	// 4. Build response theo chuẩn Laravel
	// Giả sử hàm BuildLaravelPagination của bạn nhận (context, data, total, current_page, per_page)
	response := utils.BuildLaravelPagination(c, results, total, page, perPage)

	return response, nil
}

// CreateAddress tạo địa chỉ mới
func CreateAddress(c *gin.Context, p models.AddressPayload) error {
	db, _ := utils.GetDBFromContext(c)

	// Nếu set Default, hãy bỏ Default của các địa chỉ cũ trước

	if p.IsDefault {
		db.Exec("UPDATE addresses SET is_default = 0 WHERE addressable_id = ? AND addressable_type = ?",
			p.AddressableID, p.AddressableType)
	}

	query := `INSERT INTO addresses (type, address_title, address_line_1, address_line_2, 
			  city, state_id, zip_code, country_id, phone, addressable_id, 
			  addressable_type, is_default, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`

	_, err := db.Exec(query, p.Type, p.AddressTitle, p.AddressLine1, p.AddressLine2,
		p.City, p.StateID, p.ZipCode, p.CountryID, p.Phone,
		p.AddressableID, p.AddressableType, 0)
	return err
}

func UpdateAddress(c *gin.Context, addressID string, payload models.AddressPayload) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("không thể kết nối DB: %v", err)
	}

	query := `
		UPDATE addresses 
		SET 
			type = :type,
			address_title = :address_title,
			address_line_1 = :address_line_1,
			address_line_2 = :address_line_2,
			city = :city,
			state_id = :state_id,
			zip_code = :zip_code,
			country_id = :country_id,
			phone = :phone,
			is_default = :is_default,
			updated_at = NOW()
		WHERE id = :id 
		  AND addressable_id = :addressable_id 
		  AND addressable_type = :addressable_type
	`

	arg := map[string]interface{}{
		"id":               addressID,
		"type":             payload.Type,
		"address_title":    payload.AddressTitle,
		"address_line_1":   payload.AddressLine1,
		"address_line_2":   payload.AddressLine2,
		"city":             payload.City,
		"state_id":         payload.StateID,
		"zip_code":         payload.ZipCode,
		"country_id":       payload.CountryID,
		"phone":            payload.Phone,
		"is_default":       payload.IsDefault,
		"addressable_id":   payload.AddressableID,
		"addressable_type": payload.AddressableType,
	}

	// 1. Ghi log SQL trước khi chạy
	utils.LogSQL(query, arg)

	// 2. Thực thi query với c.Request.Context()
	result, err := db.NamedExecContext(c.Request.Context(), query, arg)
	if err != nil {
		return fmt.Errorf("lỗi thực thi update: %v", err)
	}

	// 3. Kiểm tra số dòng được update
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("không tìm thấy địa chỉ hoặc bạn không có quyền sửa")
	}

	return nil
}

func DeleteAddress(c *gin.Context, addressID int, ownerID int, ownerType string) error {
	db, _ := utils.GetDBFromContext(c)

	// Sử dụng SQL Raw để xóa vật lý (Hard Delete)
	// Kiểm tra cả addressable_id để tránh xóa trộm địa chỉ người khác
	query := `DELETE FROM addresses 
              WHERE id = ? AND  addressable_type = ?`
	utils.LogSQL(query, addressID, ownerType)

	result, err := db.Exec(query, addressID, ownerType)
	if err != nil {
		return fmt.Errorf("lỗi thực thi xóa: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("không tìm thấy địa chỉ hoặc bạn không có quyền xóa")
	}

	return nil
}
