package controllers

import (
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GET /api/v2/admin/dashboard/summary
func GetDashboardSummaryHandler(c *gin.Context) {
	summary, err := services.GetDashboardSummary(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status_code": 500,
			"success":     false,
			"message":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status_code": 200,
		"success":     true,
		"message":     "Lấy dữ liệu Dashboard thành công",
		"data":        summary,
	})
}
