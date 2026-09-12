package utils

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// GetSetting Lấy giá trị Setting kiểu Chuỗi (String) - Đọc Redis trước, fallback MySQL
func GetSetting(c *gin.Context, key string, defaultValue ...string) string {
	def := ""
	if len(defaultValue) > 0 {
		def = defaultValue[0]
	}

	tenantDB, err := GetDBFromContext(c)
	tenantID := c.GetString("tenantId")
	if err != nil || tenantDB == nil || tenantID == "" {
		return def
	}

	ctx := c.Request.Context()
	cacheKey := "tenant:" + tenantID + ":settings"

	// 1. Kiểm tra Cache Redis trước (nếu RedisClient có tồn tại)
	if RedisClient != nil {
		val, err := RedisClient.HGet(ctx, cacheKey, key).Result()
		if err == nil && val != "" {
			return val // Cache HIT (Đọc cực nhanh từ RAM)
		}
	}

	// 2. Cache MISS -> Query trực tiếp từ MySQL
	var dbValue string
	query := "SELECT `value` FROM settings WHERE `key` = ? LIMIT 1"
	err = tenantDB.GetContext(ctx, &dbValue, query, key)
	if err != nil {
		return def
	}

	// 3. Lưu lại vào Redis Hash để các request sau đọc cực nhanh (TTL 24h)
	if RedisClient != nil {
		RedisClient.HSet(ctx, cacheKey, key, dbValue)
		RedisClient.Expire(ctx, cacheKey, 24*time.Hour)
	}

	return dbValue
}

// GetSettingBool Lấy giá trị Setting kiểu Boolean (true/false)
func GetSettingBool(c *gin.Context, key string, defaultValue ...bool) bool {
	def := false
	if len(defaultValue) > 0 {
		def = defaultValue[0]
	}

	strVal := GetSetting(c, key, "")
	if strVal == "" {
		return def
	}

	boolVal, err := strconv.ParseBool(strVal)
	if err != nil {
		return strVal == "1" || strVal == "on" || strVal == "yes" || strVal == "true"
	}
	return boolVal
}

// GetSettingInt Lấy giá trị Setting kiểu Số nguyên (Int)
func GetSettingInt(c *gin.Context, key string, defaultValue ...int) int {
	def := 0
	if len(defaultValue) > 0 {
		def = defaultValue[0]
	}

	strVal := GetSetting(c, key, "")
	if strVal == "" {
		return def
	}

	intVal, err := strconv.Atoi(strVal)
	if err != nil {
		return def
	}
	return intVal
}
func GetSettingFloat(c *gin.Context, key string, defaultValue float64) float64 {
	// 1. Thử lấy từ gin.Context (nếu Middleware đã load sẵn settings)
	if settings, exists := c.Get("tenant_settings"); exists {
		if mapSettings, ok := settings.(map[string]string); ok {
			if valStr, found := mapSettings[key]; found && valStr != "" {
				if valFloat, err := strconv.ParseFloat(valStr, 64); err == nil {
					return valFloat
				}
			}
		}
	}

	// 2. Nếu chưa có trong Context, Query trực tiếp từ DB Tenant
	db, err := GetDBFromContext(c)
	if err != nil {
		return defaultValue
	}

	var valStr string
	query := "SELECT value FROM settings WHERE key = ? LIMIT 1"
	err = db.QueryRowContext(c.Request.Context(), query, key).Scan(&valStr)
	if err != nil || valStr == "" {
		return defaultValue
	}

	valFloat, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultValue
	}

	return valFloat
}

// SetSetting Lưu/Cập nhật Setting vào MySQL -> Đồng thời cập nhật Redis Cache
func SetSetting(c *gin.Context, key, value string) error {
	tenantDB, err := GetDBFromContext(c)
	tenantID := c.GetString("tenantId")
	if err != nil || tenantDB == nil {
		return err
	}

	ctx := c.Request.Context()

	// 1. Ghi vào MySQL
	query := "INSERT INTO settings (`key`, `value`) VALUES (?, ?) " +
		"ON DUPLICATE KEY UPDATE `value` = VALUES(`value`)"
	_, err = tenantDB.ExecContext(ctx, query, key, value)
	if err != nil {
		return err
	}

	// 2. Cập nhật Redis Cache ngay lập tức
	if RedisClient != nil && tenantID != "" {
		cacheKey := "tenant:" + tenantID + ":settings"
		RedisClient.HSet(ctx, cacheKey, key, value)
	}

	return nil
}

type EventNotificationSetting struct {
	InApp   bool `json:"in_app"`
	Email   bool `json:"email"`
	SMS     bool `json:"sms"`
	ZaloZNS bool `json:"zalo_zns"`
}

type NotificationSettings map[string]EventNotificationSetting

// GetNotificationSettingByEvent lấy cấu hình notification cho 1 event cụ thể (vd: "payment_success", "order_created")
// Tự động tận dụng Redis Cache từ GetSetting, an toàn chống Panic nếu JSON lỗi hoặc key không tồn tại.
func GetNotificationSettingByEvent(c *gin.Context, eventName string) EventNotificationSetting {
	settingRaw := GetSetting(c, "notification_settings")
	if settingRaw == "" {
		return EventNotificationSetting{}
	}

	var settings NotificationSettings
	if err := json.Unmarshal([]byte(settingRaw), &settings); err != nil {
		LogToFile("[Setting Error] Parse notification_settings thất bại: %v", err)
		return EventNotificationSetting{}
	}

	return settings[eventName]
}
