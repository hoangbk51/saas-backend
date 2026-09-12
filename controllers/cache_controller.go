package controllers

import (
	"fmt"
	"net/http"

	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ClearTenantCache Handler xóa sạch bộ nhớ tạm (Redis) của Cửa hàng/Tenant hiện tại
func ClearTenantCache(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Thiếu thông tin Tenant ID",
		})
		return
	}

	// 1. Kiểm tra kĩ kết nối Redis
	if utils.RedisClient == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Hệ thống chưa bật Redis Cache hoặc đã được làm sạch",
		})
		return
	}

	ctx := c.Request.Context()
	pattern := "tenant:" + tenantID + ":*" // Tìm tất cả key bắt đầu bằng tenant:{tenant_id}:

	var cursor uint64
	var keysToDelete []string

	// 2. Dùng SCAN thay cho KEYS để tránh làm đơ Redis (Non-blocking)
	for {
		var keys []string
		var err error

		// Quét từng batch 100 keys
		keys, cursor, err = utils.RedisClient.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Lỗi khi quét bộ nhớ tạm: " + err.Error(),
			})
			return
		}

		keysToDelete = append(keysToDelete, keys...)

		// Khi cursor = 0 nghĩa là đã quét hết toàn bộ Redis DB
		if cursor == 0 {
			break
		}
	}

	// 3. Thực hiện xóa các key tìm thấy
	if len(keysToDelete) > 0 {
		err := utils.RedisClient.Del(ctx, keysToDelete...).Err()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Không thể xóa Cache: " + err.Error(),
			})
			return
		}
	}

	// 4. Trả kết quả thành công
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Đã xóa sạch bộ nhớ tạm (Cache) của cửa hàng thành công",
		"cleared_keys": len(keysToDelete),
	})
}

func GetTenantCacheStatsHandler(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	if tenantID == "" || utils.RedisClient == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Redis không sẵn sàng"})
		return
	}

	ctx := c.Request.Context()
	pattern := "tenant:" + tenantID + ":*"

	var keys []string
	var cursor uint64

	// 1. Quét lấy danh sách tất cả các key thuộc về Tenant
	for {
		var currentKeys []string
		var err error
		currentKeys, cursor, err = utils.RedisClient.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi khi quét cache"})
			return
		}
		keys = append(keys, currentKeys...)
		if cursor == 0 {
			break
		}
	}

	// 2. TÍNH DUNG LƯỢNG RAM BẰNG PIPELINE (Nhanh & Tối ưu network)
	var totalBytes int64 = 0

	if len(keys) > 0 {
		pipe := utils.RedisClient.Pipeline()
		cmds := make([]*redis.IntCmd, len(keys))

		// Gom tất cả câu lệnh MEMORY USAGE vào Pipeline
		for i, key := range keys {
			cmds[i] = pipe.MemoryUsage(ctx, key)
		}

		// Thực thi đồng loạt 1 lượt
		_, _ = pipe.Exec(ctx)

		// Cộng dồn dung lượng Byte của từng Key
		for _, cmd := range cmds {
			if bytes, err := cmd.Result(); err == nil {
				totalBytes += bytes
			}
		}
	}

	// 3. Trả về kết quả đã format đẹp đẽ (ví dụ: "1.42 MB")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_keys":     len(keys),
			"total_bytes":    totalBytes,
			"size_formatted": FormatBytes(totalBytes), // Trả về dạng string "1.25 MB" cho UI
		},
	})
}

// Helper đổi Byte ra dạng KB, MB, GB cho Frontend dễ hiển thị
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
