package controllers

import (
	"encoding/json"
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAllTenantSettings xử lý endpoint GET /api/v1/tenant/settings
func GetSettings(c *gin.Context) {
	results, err := services.GetAllTenantSettings(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get settings"})
		return
	}

	// Trả trực tiếp map kết quả về
	c.JSON(http.StatusOK, results)
}

func UpdateSettings(c *gin.Context) {
	// Sửa map[string]string thành map[string]json.RawMessage
	var payload map[string]json.RawMessage

	// 1. Bind dữ liệu JSON dạng key-value động
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid json format",
			"details": err.Error(), // Thêm log này để debug dễ hơn
		})
		return
	}

	// 2. Gọi Service xử lý
	if err := services.UpdateTenantSettings(c, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
