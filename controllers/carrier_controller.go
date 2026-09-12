package controllers

import (
	"net/http"
	"strconv"

	"go-saas/models"
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

type CarrierController struct {
	service *services.CarrierService
}

func NewCarrierController(service *services.CarrierService) *CarrierController {
	return &CarrierController{
		service: service,
	}
}

// GET /admin/carriers
func (ctl *CarrierController) GetCarriers(c *gin.Context) {
	resp, code, err := services.GetCarriers(c)
	if err != nil && resp == nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, resp)

}

// GET /admin/carriers/:id
func (ctl *CarrierController) GetCarierByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID không hợp lệ"})
		return
	}

	data, err := ctl.service.GetCarierByID(c, uint32(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": data})
}

// POST /admin/carriers
func (ctl *CarrierController) CreateCarier(c *gin.Context) {
	var req models.CreateCarrierReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	id, err := ctl.service.CreateCarier(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Tạo đơn vị vận chuyển thành công",
		"id":      id,
	})
}

// PUT /admin/carriers/:id
func (ctl *CarrierController) UpdateCarier(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID không hợp lệ"})
		return
	}

	var req models.UpdateCarrierReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	if err := ctl.service.UpdateCarier(c, uint32(id), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Cập nhật đơn vị vận chuyển thành công"})
}

// DELETE /admin/carriers/:id
func (ctl *CarrierController) DeleteCarier(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "ID không hợp lệ"})
		return
	}

	if err := ctl.service.DeleteCarier(c, uint32(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Xóa đơn vị vận chuyển thành công"})
}

func GetCarrierRateTable(c *gin.Context) {
	carrierID, err := strconv.Atoi(c.Param("id"))
	if err != nil || carrierID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Carrier ID không hợp lệ"})
		return
	}

	res, statusCode, _ := services.GetCarrierRateTableService(c, carrierID)
	c.JSON(statusCode, res)
}

func SaveCarrierRateTable(c *gin.Context) {
	carrierID, err := strconv.Atoi(c.Param("id"))
	if err != nil || carrierID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Carrier ID không hợp lệ"})
		return
	}

	var req models.SaveCarrierRateTableReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payload không hợp lệ: " + err.Error()})
		return
	}

	res, statusCode, _ := services.SaveCarrierRateTableService(c, carrierID, req)
	c.JSON(statusCode, res)
}
