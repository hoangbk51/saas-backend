package controllers

import (
	"net/http"
	"strconv"

	"go-saas/models"
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

type EmailTemplateController struct {
	service *services.EmailTemplateService
}

func NewEmailTemplateController() *EmailTemplateController {
	return &EmailTemplateController{
		service: services.NewEmailTemplateService(),
	}
}

// GET /api/v1/email-templates?lang=vi
func (ctl *EmailTemplateController) GetAll(c *gin.Context) {
	langCode := c.Query("lang")
	data, err := ctl.service.GetAll(c, langCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// GET /api/v1/email-templates/:id
func (ctl *EmailTemplateController) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	data, err := ctl.service.GetByID(c, uint32(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// POST /api/v1/email-templates
func (ctl *EmailTemplateController) Create(c *gin.Context) {
	var req models.CreateEmailTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := ctl.service.Create(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Tạo email template thành công",
		"id":      id,
	})
}

// PUT /api/v1/email-templates/:id
func (ctl *EmailTemplateController) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	var req models.UpdateEmailTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ctl.service.Update(c, uint32(id), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cập nhật email template thành công"})
}

// DELETE /api/v1/email-templates/:id
func (ctl *EmailTemplateController) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	if err := ctl.service.Delete(c, uint32(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Xóa email template thành công"})
}
