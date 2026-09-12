package controllers

import (
	"encoding/json"
	"go-saas/services"
	"go-saas/utils"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetPlugins(c *gin.Context) {

	// Đọc tham số phân trang & bộ lọc từ Query String
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	if page < 1 {
		page = 1
	}

	lang := c.DefaultQuery("lang", "en")
	search := c.Query("search")
	categoryID := c.Query("category_id")

	// Gọi tầng Service thực thi logic lấy dữ liệu
	results, total, err := services.GetPlugins(c, page, perPage, lang, search, categoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugins"})
		return
	}

	// Đóng gói data theo format phân trang Laravel giống hàm mẫu của bạn
	response := utils.BuildLaravelPagination(c, results, total, page, perPage)

	c.JSON(http.StatusOK, response)
}

func GetInstalledPlugins(c *gin.Context) {

	// Đọc tham số phân trang & bộ lọc từ Query String
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "15"))
	if page < 1 {
		page = 1
	}

	lang := c.DefaultQuery("lang", "en")
	search := c.Query("search")
	categoryID := c.Query("category_id")

	// Gọi tầng Service thực thi logic lấy dữ liệu
	results, total, err := services.GetInstalledPlugins(c, page, perPage, lang, search, categoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch plugins"})
		return
	}

	// Đóng gói data theo format phân trang Laravel giống hàm mẫu của bạn
	response := utils.BuildLaravelPagination(c, results, total, page, perPage)

	c.JSON(http.StatusOK, response)
}

func GetModuleFormSchemaHandler(c *gin.Context) {
	// 1. Lấy ID từ URL param
	moduleIDStr := c.Param("id")
	moduleID, err := strconv.Atoi(moduleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID module không hợp lệ"})
		return
	}

	// 2. Gọi trực tiếp hàm từ package services giống form bạn yêu cầu
	id, code, schemaBytes, err := services.GetModuleFormSchema(c, moduleID)
	if err != nil {
		if err.Error() == "không tìm thấy module hoặc module không hỗ trợ cấu hình" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống khi tải cấu hình form"})
		return
	}

	// 3. Trả kết quả về cho Frontend (Dùng json.RawMessage để ép kiểu mảng byte thành JSON thuần túy)
	c.JSON(http.StatusOK, gin.H{
		"module_id":     id,
		"module_code":   code,
		"config_schema": json.RawMessage(schemaBytes),
	})
}

// GetPluginSettings xử lý GET /api/v1/tenant/plugins/:central_id/settings
func GetPluginSettings(c *gin.Context) {
	centralIDStr := c.Param("central_id")
	centralID, err := strconv.Atoi(centralIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID plugin central không hợp lệ"})
		return
	}

	settingsBytes, err := services.GetPluginSettings(c, centralID)
	if err != nil {
		if err.Error() == "plugin chưa được cài đặt trên tenant này" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống khi lấy cấu hình"})
		return
	}

	// Trả về JSON thuần tuý cho Frontend
	c.JSON(http.StatusOK, gin.H{
		"central_id": centralID,
		"settings":   json.RawMessage(settingsBytes),
	})
}

// SavePluginSettings xử lý POST/PUT /api/v1/tenant/plugins/:central_id/settings
// (Gộp chung Store & Update vì bản chất JSON settings là ghi đè toàn bộ cấu hình mới nhất)
func SavePluginSettings(c *gin.Context) {
	centralID, err := strconv.Atoi(c.Param("central_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid central id"})
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}

	// Gọi service xử lý
	if err := services.SavePluginSettings(c, centralID, bodyBytes); err != nil {
		// Trả về trực tiếp thông điệp lỗi ngắn gọn từ Service (ví dụ: "invalid json format")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
