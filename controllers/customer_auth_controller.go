package controllers

import (
	"go-saas/services"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CustomerLoginHandler(c *gin.Context) {
	customer, err := services.CustomerLoginHandler(c)
	if err != nil {
		// Trả về response thông báo lỗi đăng nhập chuẩn
		c.JSON(http.StatusOK, gin.H{
			"status_code": 200,
			"success":     false,
			"message":     err.Error(),
		})
		return
	}
	// 6. Trả về kết quả thành công
	c.JSON(200, gin.H{
		"status_code":  200,
		"success":      true,
		"access_token": customer.FinalToken,
		"token_type":   "Bearer",
		"message":      "Login successfully",
		"customer":     customer,
	})
}

func CustomerRegisterHandler(c *gin.Context) {
	utils.LogToFile("CustomerRegisterHandler")
	err := services.CustomerRegisterHandler(c)
	if err != nil {
		// Trả về response thông báo lỗi đăng nhập chuẩn
		c.JSON(http.StatusOK, gin.H{
			"status_code": 200,
			"success":     false,
			"message":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})

}
