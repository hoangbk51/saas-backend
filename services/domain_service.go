package services

import (
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// AddDomainToTenant thêm domain mới cho tenant hiện tại
func AddDomainToTenant(c *gin.Context, domainName string) (*models.Domain, error) {
	// 1. Lấy tenant_id từ context (được gán bởi Tenant Middleware)
	tenantID := c.GetString("tenantId")
	if tenantID == "" {
		return nil, fmt.Errorf("không tìm thấy thông tin tenant_id trong context")
	}

	// 2. Làm sạch chuỗi domain (loại bỏ http://, https://, www. và trim khoảng trắng, viết thường)
	domainName = strings.TrimSpace(strings.ToLower(domainName))
	domainName = strings.TrimPrefix(domainName, "https://")
	domainName = strings.TrimPrefix(domainName, "http://")
	domainName = strings.TrimPrefix(domainName, "www.")
	domainName = strings.TrimSuffix(domainName, "/")

	if domainName == "" {
		return nil, fmt.Errorf("tên domain không hợp lệ")
	}

	// 3. Lấy DB Connection (bảng domains liên kết với tenants nằm ở Landlord/Central DB)
	// Lưu ý: Sử dụng GetMainDBFromContext hoặc GetDBFromContext tùy theo nơi đặt bảng domains
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể kết nối CSDL từ context: %v", err)
	}

	// 4. Kiểm tra xem domain này đã tồn tại trên toàn hệ thống chưa
	checkQuery := `SELECT COUNT(*) FROM domains WHERE domain = ?`
	var count int
	err = db.QueryRowContext(c, checkQuery, domainName).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("lỗi kiểm tra trùng lặp domain: %v", err)
	}
	if count > 0 {
		return nil, fmt.Errorf("domain '%s' đã được sử dụng bởi hệ thống", domainName)
	}

	// 5. Thêm domain vào CSDL
	insertQuery := `
		INSERT INTO domains (domain, tenant_id, created_at, updated_at) 
		VALUES (?, ?, NOW(), NOW())`

	res, err := db.ExecContext(c, insertQuery, domainName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("lỗi khi thêm domain vào CSDL: %v", err)
	}

	lastID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("lỗi lấy ID domain vừa tạo: %v", err)
	}

	// 6. Trả về thông tin domain vừa thêm
	domainObj := &models.Domain{
		ID:       uint(lastID),
		Domain:   domainName,
		TenantID: tenantID,
	}

	return domainObj, nil
}
