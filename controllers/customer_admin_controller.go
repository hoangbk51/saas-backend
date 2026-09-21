package controllers

import (
	"go-saas/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetCustomerDetail(c *gin.Context) {
	idStr := c.Param("id")
	//CustomerId, _ := strconv.Atoi(idStr)
	CustomerId, err := strconv.ParseInt(idStr, 10, 64)
	//CustomerId := idStr.(int64)

	// Gọi tầng dịch vụ để bốc tách dữ liệu đơn hàng
	customerDetail, statusCode, err := services.GetCustomerDetail(c, CustomerId)
	if err != nil {
		c.JSON(statusCode, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// Trả kết quả thành công rực rỡ
	c.JSON(statusCode, gin.H{
		"status": "success",
		"data":   customerDetail,
	})
}

func ListCustomers(c *gin.Context) {
	list, err := services.ListCustomersService(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func CreateCustomer(c *gin.Context) {
	res, code, err := services.CreateCustomerService(c)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func UpdateCustomer(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.UpdateCustomerService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func DeleteCustomer(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.DeleteCustomerService(c, id)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

// --- CUSTOMER HISTORY CONTROLLERS ---

func AddCustomerHistory(c *gin.Context) {
	customerID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, code, err := services.AddCustomerHistoryService(c, customerID)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func UpdateCustomerHistory(c *gin.Context) {
	historyID, _ := strconv.ParseInt(c.Param("history_id"), 10, 64)
	res, code, err := services.UpdateCustomerHistoryService(c, historyID)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}

func DeleteCustomerHistory(c *gin.Context) {
	historyID, _ := strconv.ParseInt(c.Param("history_id"), 10, 64)
	res, code, err := services.DeleteCustomerHistoryService(c, historyID)
	if err != nil && code == http.StatusInternalServerError {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, res)
}
