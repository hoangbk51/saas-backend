package services

import (
	"encoding/json"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// FetchTikTokProductsFromAPI gọi API POST /tiktok/product/search để lấy sản phẩm
func FetchTikTokProductsFromAPI(c *gin.Context, mockBaseURL string, pageSize string, reqBody map[string]interface{}) ([]models.TikTokProduct, error) {
	headers := map[string]string{
		"x-tts-access-token": c.GetHeader("Authorization"),
	}

	searchURL := fmt.Sprintf("%s/tiktok/product/search?page_size=%s", mockBaseURL, pageSize)
	respBytes, err := utils.RequestMock(c, "POST", searchURL, headers, reqBody)
	if err != nil && len(respBytes) == 0 {
		return nil, fmt.Errorf("lỗi kết nối TikTok Product Search API: %w", err)
	}

	var searchRes models.TikTokSearchProductsResponse
	if err := json.Unmarshal(respBytes, &searchRes); err != nil || searchRes.Code != 0 {
		return nil, fmt.Errorf("lỗi parse dữ liệu sản phẩm TikTok")
	}

	return searchRes.Data.Products, nil
}

// SaveOrUpdateTikTokProduct lưu hoặc cập nhật Sản phẩm & Biến thể vào MySQL
func SaveOrUpdateTikTokProduct(tx *sqlx.Tx, userID uint64, ttProduct models.TikTokProduct) error {
	var productID uint64
	utils.LogToFile("SaveOrUpdateTikTokProduct")

	// 1. Kiểm tra sản phẩm đã tồn tại theo Tên/Title hoặc ID TikTok chưa
	checkQuery := "SELECT `id` FROM `products` WHERE `user_id` = ? AND `name` = ? LIMIT 1"
	err := tx.Get(&productID, checkQuery, userID, ttProduct.Title)

	if err != nil || productID == 0 {
		// Insert sản phẩm mới
		insertProductSQL := "INSERT INTO `products` (" +
			" `title`, `active`, `created_at`, `updated_at`" +
			") VALUES ( ?, ?, NOW(), NOW())"

		activeStatus := 1
		if ttProduct.Status != "ACTIVED" {
			activeStatus = 0
		}

		res, err := tx.Exec(insertProductSQL, ttProduct.Title, activeStatus)
		if err != nil {
			return fmt.Errorf("lỗi insert product (%s): %w", ttProduct.Title, err)
		}
		newID, _ := res.LastInsertId()
		productID = uint64(newID)
	}

	// 2. Lưu/Cập nhật các biến thể SKU trong `product_variants`
	for _, sku := range ttProduct.SKUs {
		priceFloat, _ := strconv.ParseFloat(sku.Price.OriginalPrice, 64)
		stockQty := 0
		if len(sku.StockInfos) > 0 {
			stockQty = sku.StockInfos[0].AvailableStock
		}

		var variantID uint64
		checkVariantSQL := "SELECT `id` FROM `product_variants` WHERE `user_id` = ? AND `sku` = ? LIMIT 1"
		vErr := tx.Get(&variantID, checkVariantSQL, userID, sku.ID)

		if vErr == nil && variantID > 0 {
			// Cập nhật SKU đã có
			updateVariantSQL := "UPDATE `product_variants` SET `price` = ?, `quantity` = ?, `updated_at` = NOW() WHERE `id` = ?"
			_, err = tx.Exec(updateVariantSQL, priceFloat, stockQty, variantID)
			if err != nil {
				return fmt.Errorf("lỗi update product_variant (SKU: %s): %w", sku.ID, err)
			}
		} else {
			utils.LogToFile("insert SKU mới")
			// Insert SKU mới
			insertVariantSQL := "INSERT INTO `product_variants` (" +
				"`product_id`,  `sku`, `price`, `quantity`, `created_at`, `updated_at`" +
				") VALUES (?, ?, ?, ?, NOW(), NOW())"

			_, err = tx.Exec(insertVariantSQL, productID, sku.ID, priceFloat, stockQty)
			if err != nil {
				return fmt.Errorf("lỗi insert product_variant (SKU: %s): %w", sku.ID, err)
			}
		}
	}

	return nil
}
