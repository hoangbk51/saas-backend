package services

import (
	"fmt"
	"go-saas/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// Thêm vào Compare
func AddCompare(c *gin.Context, customerID int, ipAddress string, productID uint64) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	// 1. Kiểm tra trùng lặp sản phẩm trong danh sách so sánh
	var exists int
	var checkQuery string
	var checkArgs []interface{}

	if customerID > 0 {
		// Đối với User đã đăng nhập
		checkQuery = "SELECT COUNT(*) FROM compares WHERE customer_id = ? AND product_id = ?"
		checkArgs = []interface{}{customerID, productID}
	} else {
		// Đối với Guest: Check theo IP và đảm bảo customer_id IS NULL
		checkQuery = "SELECT COUNT(*) FROM compares WHERE ip_address = ? AND customer_id IS NULL AND product_id = ?"
		checkArgs = []interface{}{ipAddress, productID}
	}

	utils.LogSQL(checkQuery, checkArgs...)
	err = db.Get(&exists, checkQuery, checkArgs...)

	if err == nil && exists > 0 {
		return fmt.Errorf("sản phẩm đã có trong danh sách so sánh")
	}

	// 2. Chuẩn bị tham số customer_id (Nếu là 0 thì chuyển thành nil để lưu NULL vào DB)
	var customerParam interface{}
	if customerID > 0 {
		customerParam = customerID
	} else {
		customerParam = nil
	}

	// 3. Thực hiện Insert
	query := `INSERT INTO compares (product_id, customer_id, ip_address, created_at, updated_at) 
              VALUES (?, ?, ?, ?, ?)`
	now := time.Now()
	utils.LogSQL(query, productID, customerParam, ipAddress, now, now)

	_, err = db.Exec(query, productID, customerParam, ipAddress, now, now)
	return err
}

// Lấy danh sách Compare
func GetCompares(c *gin.Context, customerID int, ipAddress string) ([]map[string]interface{}, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}
	results := []map[string]interface{}{}

	var query string
	var args []interface{}

	if customerID > 0 {
		// User đăng nhập: Lấy theo customer_id
		query = `SELECT id, product_id, ip_address, created_at FROM compares WHERE customer_id = ? ORDER BY created_at DESC`
		args = []interface{}{customerID}
	} else {
		// Guest: Lấy theo IP và customer_id phải là NULL
		query = `SELECT id, product_id, ip_address, created_at FROM compares WHERE ip_address = ? AND customer_id IS NULL ORDER BY created_at DESC`
		args = []interface{}{ipAddress}
	}

	utils.LogSQL(query, args...)
	rows, err := db.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		row := make(map[string]interface{})
		err := rows.MapScan(row)
		if err != nil {
			continue
		}
		results = append(results, row)
	}
	return results, nil
}

// Xóa khỏi Compare
func RemoveCompare(c *gin.Context, customerID int, ipAddress string, productID uint64) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	var query string
	var args []interface{}

	if customerID > 0 {
		// Xóa theo tài khoản đăng nhập
		query = "DELETE FROM compares WHERE customer_id = ? AND product_id = ?"
		args = []interface{}{customerID, productID}
	} else {
		// Xóa theo IP của Guest
		query = "DELETE FROM compares WHERE ip_address = ? AND customer_id IS NULL AND product_id = ?"
		args = []interface{}{ipAddress, productID}
	}

	utils.LogSQL(query, args...)
	result, err := db.Exec(query, args...)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("không tìm thấy sản phẩm trong danh sách so sánh")
	}
	return nil
}
