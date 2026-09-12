package utils

import (
	"database/sql"
	"fmt"
	"time"
)

// ScanRowToMap chuyển đổi 1 dòng dữ liệu từ *sql.Rows thành map[string]interface{}
func ScanRowToMap(rows *sql.Rows) (map[string]interface{}, error) {
	// 1. Lấy danh sách tên các cột từ kết quả query
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// 2. Tạo slice chứa các interface{} để nhận dữ liệu từ Scan
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))

	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	// 3. Scan dòng hiện tại vào valuePtrs
	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	// 4. Chuyển đổi dữ liệu sang map
	rowMap := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]

		// Bớt xử lý NULL
		if val == nil {
			rowMap[col] = nil
			continue
		}

		// Xử lý các kiểu dữ liệu phổ biến từ driver MySQL/MariaDB
		switch v := val.(type) {
		case []byte:
			strVal := string(v)

			// Thử parse sang time.Time nếu có dạng datetime chuẩn
			if t, err := time.Parse("2006-01-02 15:04:05", strVal); err == nil {
				rowMap[col] = t
			} else if t, err := time.Parse("2006-01-02", strVal); err == nil {
				rowMap[col] = t
			} else {
				rowMap[col] = strVal
			}

		case time.Time:
			rowMap[col] = v

		default:
			rowMap[col] = v
		}
	}

	return rowMap, nil
}

// Helper nhỏ để hỗ trợ query NOT IN đơn giản
func SqlxInHelper(query string, optionID interface{}, ids []uint64) (string, []interface{}, error) {
	placeholders := ""
	args := []interface{}{optionID}
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}
	return fmt.Sprintf(query, placeholders), args, nil
}
