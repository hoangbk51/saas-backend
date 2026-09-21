package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CreateReviewHandler(c *gin.Context) {
	var input models.ReviewInput

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

func ListReviews(c *gin.Context) {
	list, err := services.ListReviewsService(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func GetReviewDetail(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.GetReviewDetailService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func CreateReview(c *gin.Context) {
	customerID, exists := c.Get("customerID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	res, code, err := services.CreateReviewService(c, int64(customerID.(int)))
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func UpdateReview(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.UpdateReviewService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func DeleteReview(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.DeleteReviewService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}
