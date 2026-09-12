package controllers

import (
	"go-saas/services" // Thay bằng path thực tế của bạn

	"github.com/gin-gonic/gin"
)

func AllCoupons(c *gin.Context) {

	coupons, err := services.GetAllCoupons(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": coupons})
}

func GetCouponByIds(c *gin.Context) {

	coupons, err := services.GetCouponByIds(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"data": coupons})
}

// 1. Lấy danh sách Coupon (Có phân trang)
func GetCoupons(c *gin.Context) {
	data, statusCode, err := services.GetCouponsService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

// 2. Lấy chi tiết Coupon theo ID
func GetCouponByID(c *gin.Context) {
	data, statusCode, err := services.GetCouponByIDService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

// 3. Tạo mới Coupon
func CreateCoupon(c *gin.Context) {
	data, statusCode, err := services.CreateCouponService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

// 4. Cập nhật Coupon
func UpdateCoupon(c *gin.Context) {
	data, statusCode, err := services.UpdateCouponService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}

// 5. Xóa Coupon (Soft Delete)
func DeleteCoupon(c *gin.Context) {
	data, statusCode, err := services.DeleteCouponService(c)
	if err != nil && statusCode >= 500 {
		c.JSON(statusCode, data)
		return
	}
	c.JSON(statusCode, data)
}
