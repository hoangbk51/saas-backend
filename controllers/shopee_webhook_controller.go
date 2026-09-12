package controllers

import (
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleShopeeWebhook POST /api/v1/webhooks/shopee
func HandleShopeeWebhook(c *gin.Context) {
	var payload models.ShopeeWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "Invalid payload"})
		return
	}

	// 1. Phản hồi HTTP 200 OK ngay lập tức cho Shopee trong < 1 giây
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})

	// 2. Chạy async ngầm để xử lý trừ kho / lưu DB tránh blocking Webhook HTTP Connection
	go func(p models.ShopeeWebhookPayload, ctx *gin.Context) {
		err := services.ProcessShopeeOrderWebhook(ctx, p)
		if err != nil {
			utils.LogToFile(fmt.Sprintf("[Webhook Shopee Error] OrderSN %s: %v", p.Data.OrderSN, err))
		}
	}(payload, c.Copy())
}
