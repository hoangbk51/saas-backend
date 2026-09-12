package controllers

import (
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetTenantInformationHandler(c *gin.Context) {
	// Tự động nhận diện Domain từ Request Host (ví dụ: myshop.com)
	requestDomain := c.Request.Host

	// Gọi service lấy thông tin cấu hình Tenant
	tenantInfo, err := services.GetTenantInfoByDomain(c.Request.Context(), requestDomain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Lỗi hệ thống khi lấy thông tin tenant",
		})
		return
	}

	// Nếu không tìm thấy Tenant gắn với Domain này
	if tenantInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Không tìm thấy thông tin cấu hình cho domain này",
		})
		return
	}

	// Trả về kết quả thành công
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tenantInfo,
	})
}

func TenantInit(c *gin.Context) {

	tenantID := c.GetString("tenantId")

	pluginCodes, _ := services.GetTenantPluginCodes(tenantID)

	c.JSON(200, gin.H{
		"plugins": pluginCodes,
	})
}
