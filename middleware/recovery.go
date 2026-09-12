package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// JSONRecoveryMiddleware bắt tất cả Panic xảy ra trong request và trả về JSON chuẩn
func JSONRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 1. In log Panic chi tiết ra Terminal/Console để dev debug
				log.Printf("[PANIC RECOVERED] Lỗi nghiêm trọng: %v\nStack Trace:\n%s", err, string(debug.Stack()))

				// 2. Trả về Response JSON chuẩn về cho Frontend (Tránh lỗi Unexpected end of JSON)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("Lỗi hệ thống nội bộ (Panic): %v", err),
					"error":   "Internal Server Error",
				})
			}
		}()

		// Tiếp tục chạy các Handler/Route tiếp theo
		c.Next()
	}
}
