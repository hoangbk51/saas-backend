package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

type ReturnController struct {
	returnService *services.ReturnService
}

func NewReturnController(returnService *services.ReturnService) *ReturnController {
	return &ReturnController{returnService: returnService}
}

// POST /api/v1/customer/returns
func (ctl *ReturnController) CreateCustomerReturn(c *gin.Context) {
	customerIDVal, exists := c.Get("customerID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Vui lòng đăng nhập"})
		return
	}
	customerID := customerIDVal.(int)

	var req models.CreateReturnRequest
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()})
			return
		}
	} else {
		orderID := utils.StringToInt64(c.PostForm("order_id"))
		req.OrderID = orderID
		req.Type = c.PostForm("type")
		req.Reason = c.PostForm("reason")
		req.Note = c.PostForm("note")
		_ = json.Unmarshal([]byte(c.PostForm("items")), &req.Items)
	}

	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng chọn ít nhất 1 sản phẩm cần đổi/trả"})
		return
	}

	returnID, returnCode, err := ctl.returnService.CreateReturnRequest(c, customerID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"message":     "Tạo yêu cầu đổi/trả thành công!",
		"return_id":   returnID,
		"return_code": returnCode,
	})
}

// GET /api/v1/customer/returns
func (ctl *ReturnController) GetCustomerReturns(c *gin.Context) {
	customerID := c.MustGet("customerId").(int64)

	list, err := ctl.returnService.GetCustomerReturns(c, customerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
}

// GET /api/v1/customer/returns/:id
func (ctl *ReturnController) GetCustomerReturnDetail(c *gin.Context) {
	customerID := c.MustGet("customerId").(int64)
	returnID := c.Param("id")

	detail, err := ctl.returnService.GetCustomerReturnDetail(c, customerID, returnID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": detail})
}

// GET /admin/return-requests
func (ctl *ReturnController) GetAllAdminRefund(c *gin.Context) {
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	data, total, err := ctl.returnService.GetAllAdminRefund(c, status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   data,
		"meta": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total":        total,
		},
	})
}

// GET /admin/return-requests/:id
func (ctl *ReturnController) GetAdminRefundByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	data, err := ctl.returnService.GetAdminRefundByID(c, uint32(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": data})
}

// POST /admin/return-requests
func (ctl *ReturnController) CreateAdminRefund(c *gin.Context) {
	var req models.CreateReturnRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := ctl.returnService.CreateAdminRefund(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Tạo yêu cầu trả hàng thành công",
		"id":      id,
	})
}

// PUT /admin/return-requests/:id
func (ctl *ReturnController) UpdateAdminRefund(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	var req models.UpdateReturnRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ctl.returnService.UpdateAdminRefund(c, uint32(id), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cập nhật yêu cầu trả hàng thành công"})
}

// DELETE /admin/return-requests/:id
func (ctl *ReturnController) DeleteAdminRefund(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	if err := ctl.returnService.DeleteAdminRefund(c, uint32(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xóa yêu cầu trả hàng thành công"})
}

func (ctl *ReturnController) AcceptReturn(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID không hợp lệ"})
		return
	}

	var req models.AcceptReturnRequestReq
	// Allow body rỗng nếu không truyền admin_note
	_ = c.ShouldBindJSON(&req)

	if err := ctl.returnService.AcceptReturn(c, uint32(id), req.AdminNote); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Đã duyệt yêu cầu trả hàng thành công",
	})
}
