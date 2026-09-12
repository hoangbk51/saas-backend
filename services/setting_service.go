package services

import (
	"encoding/json"
	"fmt"
	"go-saas/utils"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// GetAllTenantSettings lấy toàn bộ cài đặt của Tenant (Ưu tiên Redis Cache + Parse đúng Type)
func GetAllTenantSettings(c *gin.Context) (map[string]interface{}, error) {
	tenantID := c.GetString("tenantId")
	ctx := c.Request.Context()
	cacheKey := fmt.Sprintf("tenant:%s:settings", tenantID)

	// =========================================================================
	// 1. THỬ LẤY TỪ REDIS HASH (CACHE HIT)
	// =========================================================================
	if utils.RedisClient != nil && tenantID != "" {
		cachedMap, err := utils.RedisClient.HGetAll(ctx, cacheKey).Result()
		if err == nil && len(cachedMap) > 0 {
			resultMap := make(map[string]interface{})
			for code, rawVal := range cachedMap {
				resultMap[code] = parseSettingValue(rawVal)
			}
			return resultMap, nil
		}
	}

	// =========================================================================
	// 2. NẾU CACHE MISS: ĐỌC TỪ DATABASE MYSQL
	// =========================================================================
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		log.Printf("[Service Error] Tenant DB connection failed: %v", err)
		return nil, err
	}

	query := "SELECT code, value FROM settings"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("[Service Error] Query settings failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	settingsMap := make(map[string]interface{})
	redisFields := make(map[string]interface{})

	for rows.Next() {
		var code string
		var rawValue string

		if err := rows.Scan(&code, &rawValue); err != nil {
			log.Printf("[Service Error] Scan setting row failed: %v", err)
			return nil, err
		}

		// Lưu chuỗi thô để nạp vào Redis Hash
		redisFields[code] = rawValue

		// Parse ra đúng Data Type (Bool, Object, Number, String) để trả về cho Client
		settingsMap[code] = parseSettingValue(rawValue)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[Service Error] Rows iteration error: %v", err)
		return nil, err
	}

	// =========================================================================
	// 3. NẠP ẤM LẠI CACHE REDIS HASH (SET TTL 24 GIỜ)
	// =========================================================================
	if utils.RedisClient != nil && tenantID != "" && len(redisFields) > 0 {
		pipe := utils.RedisClient.Pipeline()
		pipe.HSet(ctx, cacheKey, redisFields)
		pipe.Expire(ctx, cacheKey, 24*time.Hour) // Tự động hết hạn sau 24h

		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("[Redis Warning] Nạp cache settings thất bại cho tenant %s: %v", tenantID, err)
		}
	}

	return settingsMap, nil
}

// =========================================================================
// HELPER: Parse chuỗi thô từ DB/Redis ra đúng kiểu dữ liệu Go/JSON
// =========================================================================
func parseSettingValue(raw string) interface{} {
	if raw == "" {
		return ""
	}

	// 1. Thử Unmarshal xem có phải là JSON Object / Array / Boolean / Number không
	var parsed interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		return parsed
	}

	// 2. Nếu không phải JSON hợp lệ -> Trả về chuỗi String nguyên bản (vd: "vi", "VND", "hai phong")
	return raw
}

// UpdateTenantSettings cập nhật loạt settings dạng key-value và invalidate Redis cache
func UpdateTenantSettings(c *gin.Context, settingsMap map[string]json.RawMessage) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		log.Printf("[Service Error] Tenant DB connection failed: %v", err)
		return err
	}

	tenantID := c.GetString("tenantId")

	// 1. Khởi tạo Transaction
	ctx := c.Request.Context()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[Service Error] Begin transaction failed: %v", err)
		return err
	}
	defer tx.Rollback()

	// 2. Chuẩn bị câu lệnh UPDATE (Dùng WHERE code = ? hoặc key = ? tùy schema của bạn)
	query := "UPDATE settings SET value = ? WHERE code = ?"
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		log.Printf("[Service Error] Prepare statement failed: %v", err)
		return err
	}
	defer stmt.Close()

	// 3. Duyệt qua từng cặp code - rawValue để chuẩn hóa và UPDATE
	for code, rawValue := range settingsMap {
		var cleanValue string

		// Thử unmarshal xem có phải là JSON string có bao ngoặc kép không (vd: "\"vi\"")
		var strVal string
		if err := json.Unmarshal(rawValue, &strVal); err == nil {
			// Nếu Unmarshal thành công -> Nó là String đơn thuần (vd: "vi", "VND", "20")
			cleanValue = strVal
		} else {
			// Nếu Unmarshal ra string bị lỗi -> Nó là Object/Array/Boolean JSON (vd: {"order_created":...})
			// Giữ nguyên nguyên bản chuỗi JSON để lưu vào DB
			cleanValue = string(rawValue)
		}

		_, err := stmt.ExecContext(ctx, cleanValue, code)
		if err != nil {
			log.Printf("[Service Error] Update failed for code %s: %v", code, err)
			return err
		}
	}

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		log.Printf("[Service Error] Commit transaction failed: %v", err)
		return err
	}

	// 5. XÓA CACHE REDIS (Cache Invalidation)
	// Xóa sạch Hash settings của Tenant để lần gọi GetSetting tới tự động load lại DB
	if utils.RedisClient != nil && tenantID != "" {
		cacheKey := "tenant:" + tenantID + ":settings"
		if err := utils.RedisClient.Del(ctx, cacheKey).Err(); err != nil {
			log.Printf("[Redis Warning] Xóa cache settings thất bại cho tenant %s: %v", tenantID, err)
		} else {
			log.Printf("[Redis Success] Đã xóa cache settings cho tenant: %s", tenantID)
		}
	}

	return nil
}
