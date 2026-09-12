package services

import (
	"fmt"
	"go-saas/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// Thêm vào Wishlist
func AddWishlist(c *gin.Context, customerID int, ipAddress string, productID uint64) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	// 1. Kiểm tra trùng lặp sản phẩm trong Wishlist
	var exists int
	var checkQuery string
	var checkArgs []interface{}

	if customerID > 0 {
		// Nếu đã đăng nhập: Check theo khách hàng
		checkQuery = "SELECT COUNT(*) FROM wishlists WHERE customer_id = ? AND product_id = ?"
		checkArgs = []interface{}{customerID, productID}
	} else {
		// Nếu là Guest: Check theo IP và đảm bảo customer_id là NULL (để tránh check nhầm vào sản phẩm của user khác từng dùng chung IP)
		checkQuery = "SELECT COUNT(*) FROM wishlists WHERE ip_address = ? AND customer_id IS NULL AND product_id = ?"
		checkArgs = []interface{}{ipAddress, productID}
	}

	utils.LogSQL(checkQuery, checkArgs...)
	err = db.Get(&exists, checkQuery, checkArgs...)

	if err == nil && exists > 0 {
		return fmt.Errorf("sản phẩm đã có trong danh sách yêu thích")
	}

	// 2. Chuẩn bị tham số để Insert
	var customerParam interface{}
	if customerID > 0 {
		customerParam = customerID
	} else {
		customerParam = nil // Lưu NULL xuống Database cho Guest
	}

	// 3. Thực hiện Insert thêm trường ip_address
	query := `INSERT INTO wishlists (product_id, customer_id, ip_address, created_at, updated_at) 
              VALUES (?, ?, ?, ?, ?)`
	now := time.Now()
	utils.LogSQL(query, productID, customerParam, ipAddress, now, now)

	_, err = db.Exec(query, productID, customerParam, ipAddress, now, now)
	return err
}

// Lấy danh sách Wishlist
// Lấy danh sách Wishlist (Hỗ trợ cả User và Guest qua IP)
func GetWishlists(c *gin.Context, customerID int, ipAddress string) ([]map[string]interface{}, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}
	results := []map[string]interface{}{}

	var query string
	var args []interface{}

	// Rẽ nhánh Query tùy theo đối tượng
	if customerID > 0 {
		// Đối với User đã đăng nhập: Lấy theo customer_id
		query = `SELECT id, product_id, ip_address, created_at 
                 FROM wishlists 
                 WHERE customer_id = ? 
                 ORDER BY created_at DESC`
		args = []interface{}{customerID}
	} else {
		// Đối với Guest: Lấy theo ip_address và bắt buộc customer_id phải là NULL
		// để tránh lấy nhầm dữ liệu cũ của user khác trùng IP sau này
		query = `SELECT id, product_id, ip_address, created_at 
                 FROM wishlists 
                 WHERE ip_address = ? AND customer_id IS NULL 
                 ORDER BY created_at DESC`
		args = []interface{}{ipAddress}
	}

	utils.LogSQL(query, args...)

	// Thực thi câu lệnh SQL với các tham số tương ứng
	rows, err := db.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		row := make(map[string]interface{})
		err := rows.MapScan(row)
		if err != nil {
			continue // Bỏ qua dòng lỗi nếu có
		}

		// Convert kiểu dữ liệu dạng []byte từ DB (như created_at) về dạng readable nếu cần thiết
		results = append(results, row)
	}

	return results, nil
}

// Xóa khỏi Wishlist
func RemoveWishlist(c *gin.Context, customerID int, ipAddress string, productID uint64) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	var query string
	var args []interface{}

	// Rẽ nhánh điều kiện xóa dựa trên trạng thái đăng nhập
	if customerID > 0 {
		// Đối với User đã đăng nhập: Xóa chính xác theo customer_id của họ
		query = "DELETE FROM wishlists WHERE customer_id = ? AND product_id = ?"
		args = []interface{}{customerID, productID}
	} else {
		// Đối với Guest: Xóa theo IP và chỉ xóa những dòng của Guest (customer_id IS NULL)
		query = "DELETE FROM wishlists WHERE ip_address = ? AND customer_id IS NULL AND product_id = ?"
		args = []interface{}{ipAddress, productID}
	}

	utils.LogSQL(query, args...)
	result, err := db.Exec(query, args...)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("không tìm thấy sản phẩm trong wishlist")
	}
	return nil
}
