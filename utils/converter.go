package utils

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/jmoiron/sqlx"
)

// StringToInt64 chuyển đổi chuỗi sang số nguyên 64-bit
func StringToInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0 // Hoặc xử lý lỗi tùy bạn, ở đây trả về 0 cho an toàn
	}
	return i
}

func ParseToFloat(val interface{}) float64 {
	if val == nil {
		return 0.0
	}

	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case []byte: // Drivers MySQL thường trả về kiểu decimal dưới dạng []byte
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return 0.0
		}
		return f
	case string: // Một số driver trả về kiểu string
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0.0
		}
		return f
	default:
		// Thử ép kiểu qua Sprintf phòng trường hợp kiểu dữ liệu lạ
		f, err := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		if err != nil {
			return 0.0
		}
		return f
	}
}

// GetStringValue trả về chuỗi string từ *string, nếu nil thì trả về chuỗi rỗng ""
func GetStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetStringValueOrDefault trả về chuỗi string từ *string, nếu nil thì trả về defaultValue
func GetStringValueOrDefault(s *string, defaultValue string) string {
	if s == nil {
		return defaultValue
	}
	return *s
}

// NullStringToString chuyển từ sql.NullString sang string an toàn
func NullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func InterfaceToInt(val interface{}) int {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint32:
		return int(v)
	case float64:
		return int(v)
	case []byte:
		i, _ := strconv.Atoi(string(v))
		return i
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

func InterfaceToFloat(val interface{}) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

// InQuery Helper wrapper cho sqlx.In
func InQuery(query string, params ...interface{}) (string, []interface{}, error) {
	return sqlx.In(query, params...)
}

func ParseIntOrDefault(val string, defaultValue int) int {
	if val == "" {
		return defaultValue
	}
	res, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return res
}

// ParseInt64OrDefault chuyển đổi string sang int64, trả về defaultValue nếu lỗi hoặc rỗng
func ParseInt64OrDefault(val string, defaultValue int64) int64 {
	if val == "" {
		return defaultValue
	}
	res, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultValue
	}
	return res
}
