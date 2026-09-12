package controllers

import (
	"go-saas/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreateReviewHandler(c *gin.Context) {
	var input services.ReviewInput

	// 1. Bind JSON từ request
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Dữ liệu không hợp lệ",
			"success": false,
		})
		return
	}

	// 2. Logic kiểm tra Auth (Giống Auth::guard('sanctum'))
	// Giả sử Middleware của bạn đã set "customerId" vào context nếu token hợp lệ
	customerID, exists := c.Get("customerID")
	if !exists {
		customerID = 0
	}

	input.CustomerID = customerID.(int)

	// 3. Gọi service để lưu
	if err := services.CreateReview(c, input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Lỗi hệ thống",
			"success": false,
		})
		return
	}

	// 4. Trả về kết quả (Giống trans('review successfully'))
	c.JSON(http.StatusOK, gin.H{
		"message": "Đánh giá thành công",
		"success": true,
	})
}
