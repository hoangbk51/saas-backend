package controllers

import (
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

func GetUsers(c *gin.Context) {
	data, statusCode, err := services.GetUsersService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func GetUserByID(c *gin.Context) {
	data, statusCode, err := services.GetUserByIDService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func CreateUser(c *gin.Context) {
	data, statusCode, err := services.CreateUserService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func UpdateUser(c *gin.Context) {
	data, statusCode, err := services.UpdateUserService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

func DeleteUser(c *gin.Context) {
	data, statusCode, err := services.DeleteUserService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}
