package controllers

import (
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

func GetRoles(c *gin.Context) {
	data, statusCode, err := services.GetRolesService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func GetRoleByID(c *gin.Context) {
	data, statusCode, err := services.GetRoleByIDService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func CreateRole(c *gin.Context) {
	data, statusCode, err := services.CreateRoleService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func UpdateRole(c *gin.Context) {
	data, statusCode, err := services.UpdateRoleService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func DeleteRole(c *gin.Context) {
	data, statusCode, err := services.DeleteRoleService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}
