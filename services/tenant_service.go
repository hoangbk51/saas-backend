package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"go-saas/utils"
)

func GetTenantInfoByDomain(ctx context.Context, rawDomain string) (json.RawMessage, error) {
	db, err := utils.GetCentralDB()
	if err != nil {
		return nil, err
	}

	// Làm sạch domain (bỏ http://, https://, /)
	cleanedDomain := utils.CleanDomain(rawDomain)

	// Câu lệnh SQL Join giữa bảng tenants và domains thông qua tenant_id
	query := `
		SELECT t.data 
		FROM tenants t
		INNER JOIN domains d ON t.id = d.tenant_id
		WHERE d.domain = ? LIMIT 1
	`

	var dataStr string // Biến tạm để quét chuỗi JSON từ DB

	err = db.QueryRowContext(ctx, query, cleanedDomain).Scan(&dataStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Không tìm thấy domain này trong hệ thống
		}
		return nil, err
	}

	return json.RawMessage(dataStr), nil
}
