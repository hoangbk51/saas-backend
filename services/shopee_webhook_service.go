package services

import (
	"fmt"
	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// ProcessShopeeOrderWebhook xử lý logic nghiệp vụ khi có Webhook
func ProcessShopeeOrderWebhook(c *gin.Context, payload models.ShopeeWebhookPayload) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("lỗi kết nối database: %w", err)
	}

	orderSN := payload.Data.OrderSN
	status := payload.Data.OrderStatus

	// 1. Tìm thông tin User/Shop dựa trên ShopID từ Shopee
	var channelCredential struct {
		UserID uint64 `db:"user_id"`
		ID     uint64 `db:"id"`
	}
	findShopQuery := `SELECT id, user_id FROM user_channels WHERE shop_id = ? AND channel_name = 'shopee' LIMIT 1`
	err = db.Get(&channelCredential, findShopQuery, fmt.Sprintf("%d", payload.ShopID))
	if err != nil {
		return fmt.Errorf("không tìm thấy shop kết nối với shop_id %d: %w", payload.ShopID, err)
	}

	userID := channelCredential.UserID

	// 2. Kiểm tra xem đơn hàng đã được lưu trong DB chưa
	var existingOrder struct {
		ID     uint64 `db:"id"`
		Status string `db:"status"`
	}
	checkOrderQuery := `SELECT id, status FROM orders WHERE channel_order_sn = ? AND user_id = ? LIMIT 1`
	err = db.Get(&existingOrder, checkOrderQuery, orderSN, userID)

	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// CASE 1: Đơn Hủy -> Hoàn lại kho nếu trước đó đã trừ kho
	if status == "CANCELLED" {
		if existingOrder.ID > 0 && existingOrder.Status != "CANCELLED" {
			// Cập nhật trạng thái đơn
			_, _ = tx.Exec(`UPDATE orders SET status = 'CANCELLED', updated_at = NOW() WHERE id = ?`, existingOrder.ID)

			// Hoàn trả kho (AddStock) cho các sản phẩm trong đơn
			err := restoreOrderStock(tx, userID, existingOrder.ID)
			if err != nil {
				return fmt.Errorf("lỗi hoàn kho khi hủy đơn: %w", err)
			}
		}
		return tx.Commit()
	}

	// CASE 2: Đơn mới phát sinh (READY_TO_SHIP / PROCESSED) -> Trừ Kho
	if existingOrder.ID == 0 {
		// Kéo chi tiết đơn từ Shopee (dùng lại helper đã viết trước đó)
		shopeeOrders, err := fetchOrdersFromShopeeAPI(getShopeeMockBase(c), "", fmt.Sprintf("%d", payload.ShopID), 1)
		if err != nil || len(shopeeOrders) == 0 {
			return fmt.Errorf("không thể lấy thông tin chi tiết đơn %s từ Shopee", orderSN)
		}

		chOrder := shopeeOrders[0]
		chOrder.ChannelID = channelCredential.ID

		// Tạo đơn hàng mới trong DB
		err = insertOrderRecord(tx, userID, chOrder)
		if err != nil {
			return fmt.Errorf("lỗi lưu đơn hàng: %w", err)
		}

		// TRỪ KHO TỨC THỜI (Deduct Stock) cho từng mặt hàng
		for _, item := range chOrder.Items {
			if item.VariantID > 0 {
				err := DeductStock(tx, item.VariantID, item.Quantity)
				if err != nil {
					// Log cảnh báo nếu kho Web không đủ trừ, vẫn giữ đơn hàng
					utils.LogToFile(fmt.Sprintf("Tồn kho Web không đủ cho VariantID %d (Đơn %s): %v", item.VariantID, orderSN, err))
				}
			}
		}
	} else {
		// Đơn đã có, chỉ cập nhật trạng thái mới nhất
		mappedStatus := mapShopeeStatusToWeb(status)
		_, _ = tx.Exec(`UPDATE orders SET status = ?, updated_at = NOW() WHERE id = ?`, mappedStatus, existingOrder.ID)
	}

	return tx.Commit()
}

// Dùng cho Webhook Đơn hàng mới / Đơn Web mới
func DeductStock(tx *sqlx.Tx, variantID uint64, boughtQty int) error {
	// Trừ kho trong product_variants
	uQuery := `UPDATE product_variants SET quantity = quantity - ? WHERE id = ? AND quantity >= ?`
	utils.LogSQL(uQuery, boughtQty, variantID, boughtQty)
	res, err := tx.Exec(uQuery, boughtQty, variantID, boughtQty)
	if err != nil {
		return err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("tồn kho không đủ để trừ")
	}

	// Trừ kho trong manage_stocks
	sQuery := `UPDATE manage_stocks SET quantity = quantity - ? WHERE variant_id = ?`
	utils.LogSQL(sQuery, boughtQty, variantID)
	_, err = tx.Exec(sQuery, boughtQty, variantID)
	return err
}

// Helper hoàn trả tồn kho khi đơn bị Hủy (AddStock)
func restoreOrderStock(tx *sqlx.Tx, userID uint64, orderID uint64) error {
	type OrderItem struct {
		VariantID uint64 `db:"variant_id"`
		Quantity  int    `db:"quantity"`
	}

	var items []OrderItem
	query := `SELECT variant_id, quantity FROM order_items WHERE order_id = ?`
	err := tx.Select(&items, query, orderID)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.VariantID > 0 {
			// Cộng lại kho vào product_variants
			_, _ = tx.Exec(`UPDATE product_variants SET quantity = quantity + ? WHERE id = ?`, item.Quantity, item.VariantID)
			// Cộng lại kho vào manage_stocks
			_, _ = tx.Exec(`UPDATE manage_stocks SET quantity = quantity + ? WHERE variant_id = ?`, item.Quantity, item.VariantID)
		}
	}
	return nil
}
