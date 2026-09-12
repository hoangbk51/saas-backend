package controllers

import (
	"fmt"
	"go-saas/services"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandleEvent tiếp nhận POST /api/v2/goship/event
func HandleGoshipEvent(c *gin.Context) {
	var payload services.GoShipWebhookPayload

	// 1. Parse JSON Request Body
	if err := c.ShouldBindJSON(&payload); err != nil {
		utils.LogToFile("[GoShip Webhook] Lỗi Bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"status":  "error",
			"message": "Invalid JSON payload",
		})
		return
	}

	utils.LogToFile(fmt.Sprintf("[GoShip Webhook] Receive Event: Code=%s | Status=%s", payload.Code, payload.Status))

	// 2. Gọi Service xử lý Business Logic
	if err := services.ProcessWebhookEvent(c, payload); err != nil {
		utils.LogToFile("[GoShip Webhook] Process Error: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// 3. Trả về phản hồi cho GoShip
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"status":  "success",
		"message": "Webhook processed successfully",
	})
}
