package services

import (
	"go-saas/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetCustomerTryOnHistory(c *gin.Context, customerID int) (utils.LaravelCollection, error) {
	db, _ := utils.GetDBFromContext(c)

	// 1. Lấy tham số phân trang từ URL (mặc định page 1, 15 item/trang)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage

	// 2. Query tính tổng số bản ghi try-on hợp lệ của customer
	var total int
	countQuery := `
		SELECT COUNT(*) 
		FROM try_on_requests 
		WHERE customer_id = ? AND result_image IS NOT NULL AND result_image != ''`

	err := db.Get(&total, countQuery, customerID)
	if err != nil {
		return utils.LaravelCollection{}, err
	}

	// 3. Query lấy danh sách dữ liệu có phân trang
	query := `
		SELECT 
			id, 
			product_id, 
			customer_id, 
			COALESCE(provider_id, '') AS provider_id, 
			COALESCE(result_image, '') AS result_image, 
			COALESCE(status, 0) AS status, 
			created_at, 
			updated_at 
		FROM try_on_requests 
		WHERE customer_id = ? AND result_image IS NOT NULL AND result_image != ''
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`

	utils.LogSQL(query, customerID, perPage, offset)

	results := []map[string]interface{}{}

	rows, err := db.Queryx(query, customerID, perPage, offset)
	if err != nil {
		return utils.LaravelCollection{}, err
	}
	defer rows.Close()

	for rows.Next() {
		row := make(map[string]interface{})
		err := rows.MapScan(row)
		if err != nil {
			return utils.LaravelCollection{}, err
		}

		// Chuyển đổi byte slice ([]byte) sang string nếu có
		for k, v := range row {
			if b, ok := v.([]byte); ok {
				row[k] = string(b)
			}
		}
		results = append(results, row)
	}

	// 4. Build response theo chuẩn phân trang Laravel
	response := utils.BuildLaravelPagination(c, results, total, page, perPage)

	return response, nil
}
