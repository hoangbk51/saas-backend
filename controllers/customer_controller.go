package controllers

import (
	"go-saas/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func LogoutCustomer(c *gin.Context) {
	// Lấy header từ request
	authHeader := c.GetHeader("Authorization")

	// Gọi service xử lý
	err := services.LogoutCustomer(c, authHeader)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Đăng xuất thất bại",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Đã xóa token và đăng xuất thành công",
	})
}

func CustomerVerifyHandler(c *gin.Context) {
	// Lấy token từ URL: /verify?token=abcxyz...
	token := c.Query("token")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Thiếu mã xác thực",
		})
		return
	}

	err := services.VerifyCustomer(c, token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// 3. Trả về thành công
	// Bạn có thể redirect khách hàng về trang login của Vue hoặc trả về JSON
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Tài khoản của bạn đã được kích hoạt thành công!",
	})
}

func CustomerOrderDetail(c *gin.Context) {
	idStr := c.Param("id")
	OrderId, _ := strconv.Atoi(idStr)

	// Gọi tầng dịch vụ để bốc tách dữ liệu đơn hàng
	orderDetail, err := services.GetOrderDetail(c, OrderId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// Trả kết quả thành công rực rỡ
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   orderDetail,
	})
}
