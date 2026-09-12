package middleware

import (
	"net/http"
	"strings"

	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func ChatAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Default = guest
		c.Set("chatRole", "guest")
		c.Set("customerID", int64(0))
		c.Set("adminID", int64(0))
		c.Set("isSuperAdmin", false)

		authHeader := c.GetHeader("Authorization")

		// WebSocket không gửi Authorization header từ browser,
		// nên hỗ trợ token qua query:
		// ?token=guest_token
		// ?token=133|personal_access_token
		if authHeader == "" {
			queryToken := c.Query("token")

			if queryToken != "" {
				// Personal access token của Customer/Admin
				if strings.Contains(queryToken, "|") {
					authHeader = "Bearer " + queryToken
				} else {
					// Guest token
					c.Set("chatGuestToken", queryToken)

					utils.LogToFile(
						"[CHAT AUTH] %s %s | role=guest reason=query_token",
						c.Request.Method,
						c.Request.URL.Path,
					)

					c.Next()
					return
				}
			}
		}

		// Không có Authorization => guest mặc định
		if authHeader == "" {
			utils.LogToFile(
				"[CHAT AUTH] %s %s | role=guest reason=no_authorization",
				c.Request.Method,
				c.Request.URL.Path,
			)

			c.Next()
			return
		}

		rawToken := strings.TrimSpace(
			strings.TrimPrefix(authHeader, "Bearer "),
		)

		parts := strings.SplitN(rawToken, "|", 2)

		if len(parts) != 2 {
			utils.LogToFile(
				"[CHAT AUTH] invalid token format",
			)

			c.JSON(
				http.StatusUnauthorized,
				gin.H{
					"error": "Invalid token format",
				},
			)
			c.Abort()
			return
		}

		tokenID := parts[0]
		plainToken := parts[1]

		utils.LogToFile(
			"[CHAT AUTH] tokenID=%s",
			tokenID,
		)

		hashedToken := utils.HashToken(plainToken)

		db, err := utils.GetDBFromContext(c)
		if err != nil {
			utils.LogToFile(
				"[CHAT AUTH] database error: %v",
				err,
			)

			c.JSON(
				http.StatusInternalServerError,
				gin.H{
					"error": "Database connection error",
				},
			)
			c.Abort()
			return
		}

		var (
			tokenableType string
			tokenableID   int64
		)

		err = db.QueryRow(
			`
			SELECT
				tokenable_type,
				tokenable_id
			FROM personal_access_tokens
			WHERE id = ?
			  AND token = ?
			  AND tokenable_type IN (
				'App\\Models\\Tenant\\Customer',
				'App\\Models\\Tenant\\User',
				'App\\Models\\Central\\Client'
			  )
			LIMIT 1
			`,
			tokenID,
			hashedToken,
		).Scan(
			&tokenableType,
			&tokenableID,
		)

		if err != nil {
			utils.LogToFile(
				"[CHAT AUTH] token lookup failed tokenID=%s error=%v",
				tokenID,
				err,
			)

			c.JSON(
				http.StatusUnauthorized,
				gin.H{
					"error": "Invalid or expired token",
				},
			)
			c.Abort()
			return
		}

		utils.LogToFile(
			"[CHAT AUTH] token valid tokenID=%s type=%s id=%d",
			tokenID,
			tokenableType,
			tokenableID,
		)

		switch tokenableType {

		case "App\\Models\\Tenant\\Customer":

			c.Set("chatRole", "customer")
			c.Set("customerID", tokenableID)

			utils.LogToFile(
				"[CHAT AUTH] role=customer customerID=%d",
				tokenableID,
			)

		case "App\\Models\\Tenant\\User":

			c.Set("chatRole", "admin")
			c.Set("adminID", tokenableID)
			c.Set("isSuperAdmin", false)

			utils.LogToFile(
				"[CHAT AUTH] role=admin adminID=%d",
				tokenableID,
			)

		case "App\\Models\\Central\\Client":

			c.Set("chatRole", "admin")
			c.Set("adminID", tokenableID)
			c.Set("isSuperAdmin", true)

			utils.LogToFile(
				"[CHAT AUTH] role=superadmin adminID=%d",
				tokenableID,
			)

		default:

			utils.LogToFile(
				"[CHAT AUTH] unsupported token type=%s",
				tokenableType,
			)

			c.JSON(
				http.StatusUnauthorized,
				gin.H{
					"error": "Unsupported token type",
				},
			)

			c.Abort()
			return
		}

		c.Next()
	}
}
