package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// POST /api/v2/admin/domains
func AddDomainHandler(c *gin.Context) {
	var req models.AddDomainRequest

	// Validate body request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status_code": 400,
			"success":     false,
			"message":     "Tên domain không được để trống",
		})
		return
	}

	// Gọi service thêm domain
	domainObj, err := services.AddDomainToTenant(c, req.Domain)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status_code": 400,
			"success":     false,
			"message":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status_code": 200,
		"success":     true,
		"message":     "Thêm domain thành công",
		"data":        domainObj,
	})
}
