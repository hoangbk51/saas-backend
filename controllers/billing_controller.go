package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetBillingHistoryHandler(c *gin.Context) {
	var filter models.BillingFilter

	// Tự động bind query parameters và kiểm tra validation (tenant_id buộc phải có)
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Thiếu thông tin tenant_id hoặc tham số không hợp lệ",
		})
		return
	}

	// Gọi service và truyền request context
	requestDomain := c.Request.Host

	// Gửi requestDomain vào service để tự động truy vết ra tenant_id
	data, err := services.GetBillingHistoryByDomain(c, requestDomain, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Lỗi hệ thống khi truy vấn lịch sử hóa đơn",
		})
		return
	}

	// Trả về dữ liệu
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}
