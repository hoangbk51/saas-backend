package controllers

import (
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// API 1: GET /api/v2/reports/stock/grouped
func GetStockProduct(c *gin.Context) {

	data, err := services.GetStockProduct(c)
	if err != nil {
		utils.LogToFile("Lỗi GetStockGroupedHandler: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// API 2: GET /api/v2/reports/stock/detail
func GetStockVariant(c *gin.Context) {

	data, err := services.GetStockVariant(c)
	if err != nil {
		utils.LogToFile("Lỗi GetStockDetailHandler: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func GetCustomerOrderReportHandler(c *gin.Context) {
	var filter models.CustomerOrderReportFilter

	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tham số truyền vào không hợp lệ",
		})
		return
	}

	// Truyền trực tiếp context `c` vào service
	paginatedResult, err := services.GetCustomerOrderReport(c, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, paginatedResult)
}

func GetProductsSellReportHandler(c *gin.Context) {
	var filter models.ProductSellReportFilter

	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tham số tìm kiếm không hợp lệ",
		})
		return
	}

	result, err := services.GetProductsSellReport(c, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func GetProductsViewReportHandler(c *gin.Context) {
	var filter models.ProductViewReportFilter

	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tham số tìm kiếm không hợp lệ",
		})
		return
	}

	result, err := services.GetProductsViewReport(c, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func GetSalesReportHandler(c *gin.Context) {
	var filter models.SalesReportFilter

	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tham số tìm kiếm không hợp lệ",
		})
		return
	}

	result, err := services.GetSalesReport(c, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

func GetCouponsReportHandler(c *gin.Context) {
	var filter models.CouponReportFilter

	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Tham số tìm kiếm không hợp lệ",
		})
		return
	}

	result, err := services.GetCouponsReport(c, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
