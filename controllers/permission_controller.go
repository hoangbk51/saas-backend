package controllers

import (
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

func GetGroupedPermissions(c *gin.Context) {
	data, statusCode, err := services.GetGroupedPermissionsService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}
