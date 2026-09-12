package controllers

import (
	"database/sql"
	"go-saas/models"
	"go-saas/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetPageHistories(c *gin.Context) {
	data, err := services.GetAllPageHistories(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, data)
}

func StorePageHistory(c *gin.Context) {
	var input models.PageHistory
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	err := services.CreatePageHistory(c, &input)
	if err != nil {
		c.JSON(500, gin.H{"error": "Không thể lưu lịch sử trang"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã lưu bản sao lịch sử"})
}

func UpdatePageHistory(c *gin.Context) {
	id := c.Param("id")
	var input models.PageHistory

	// Parse JSON body
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	// Thực hiện update
	err := services.UpdatePageHistory(c, id, &input)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy bản ghi để cập nhật"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống khi cập nhật"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cập nhật lịch sử trang thành công",
		"id":      id,
	})
}

func ShowPageHistory(c *gin.Context) {
	id := c.Param("id")
	history, err := services.GetPageHistoryByID(c, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "Không tìm thấy bản ghi"})
		return
	}
	c.JSON(200, history)
}

func DestroyPageHistory(c *gin.Context) {
	id := c.Param("id")
	if err := services.DeletePageHistory(c, id); err != nil {
		c.JSON(500, gin.H{"error": "Xóa thất bại"})
		return
	}
	c.JSON(200, gin.H{"message": "Đã xóa bản ghi"})
}

func GetLatestHistory(c *gin.Context) {
	pageThemeID := c.Param("id")

	// Chuyển string sang int64
	themeIDTmp, _ := strconv.ParseInt(pageThemeID, 10, 64)

	data, err := services.GetLatestPageData(c, themeIDTmp)
	if err != nil {
		c.JSON(500, gin.H{"error": "Lỗi truy vấn dữ liệu"})
		return
	}

	if data == nil {
		c.JSON(404, gin.H{"message": "Chưa có bản ghi lịch sử nào"})
		return
	}

	// Trả về trực tiếp, 'data' sẽ hiển thị đúng cấu trúc Tree Object
	c.JSON(200, gin.H{
		"page_theme_id": themeIDTmp,
		"elements":      data,
	})
}
