package controllers

import (
	"fmt"
	"go-saas/services"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SyncTikTokProductsHandler xử lý request POST /api/v1/tiktok/sync-products
func SyncTikTokProductsHandler(c *gin.Context) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi kết nối CSDL"})
		return
	}
	userID := c.GetUint64("user_id")

	var reqBody map[string]interface{}
	_ = c.ShouldBindJSON(&reqBody) // Ví dụ: {"status": "ACTIVED"}
	pageSize := c.DefaultQuery("page_size", "10")

	// 1. Lấy danh sách sản phẩm từ TikTok API Mockup
	mockBase := getTikTokMockBase(c)
	products, err := services.FetchTikTokProductsFromAPI(c, mockBase, pageSize, reqBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}

	if len(products) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Không có sản phẩm nào cần đồng bộ", "synced_count": 0})
		return
	}

	// 2. Mở Transaction CSDL để ghi sản phẩm & SKU biến thể
	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi khởi tạo DB Transaction"})
		return
	}
	defer tx.Rollback()

	syncedCount := 0
	for _, product := range products {
		if err := services.SaveOrUpdateTikTokProduct(tx, userID, product); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": fmt.Sprintf("Lỗi lưu sản phẩm %s: %s", product.Title, err.Error())})
			return
		}
		syncedCount++
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "Lỗi commit CSDL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":         0,
		"message":      "Đồng bộ sản phẩm TikTok thành công",
		"synced_count": syncedCount,
		"data":         products,
	})
}
