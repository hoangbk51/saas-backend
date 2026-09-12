package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// POST /api/v2/sso/exchange-token
func ExchangeSSOTokenHandler(c *gin.Context) {
	var req models.SSOExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Payload không hợp lệ, yêu cầu có trường 'code'",
		})
		return
	}

	// Lấy tenant_id hiện tại từ Context (Middleware domain/subdomain truyền vào)

	currentTenantID := c.GetString("tenantId")

	data, err := services.ExchangeSSOToken(c, req.Code, currentTenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SSOExchangeResponse{
		Status:  "success",
		Message: "Xác thực SSO thành công",
		Data:    *data,
	})
}
