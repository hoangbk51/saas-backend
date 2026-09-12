package controllers

import (
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func AdminLoginHandler(c *gin.Context) {
	user, err := services.AdminLoginHandler(c)
	if err != nil {
		// Trả về response thông báo lỗi đăng nhập chuẩn
		c.JSON(http.StatusOK, gin.H{
			"status_code": 200,
			"success":     false,
			"message":     err.Error(),
		})
		return
	}

	// Đăng nhập thành công
	c.JSON(http.StatusOK, gin.H{
		"status_code":    200,
		"success":        true,
		"access_token":   user.FinalToken,
		"token_type":     "Bearer",
		"message":        "Login successfully",
		"is_super_admin": user.IsSuperAdmin, // Trả về ở root JSON
		"data":           user,
	})
}
