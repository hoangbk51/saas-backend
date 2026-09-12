package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// Sinh ngẫu nhiên chuỗi token ngẫu nhiên (60 ký tự)
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func ExchangeSSOToken(c *gin.Context, code string, currentTenantID string) (*models.SSOExchangeData, error) {
	db, err := utils.GetCentralDB()
	if err != nil {
		return nil, err
	}

	// 1. Khởi tạo DB Transaction chống Race Condition
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("không thể khởi tạo transaction: %v", err)
	}
	defer tx.Rollback()

	// 2. Query và Lock dòng sso_code (FOR UPDATE)
	var sso models.SSOCodeModel
	queryCode := `
		SELECT id, code, client_id, tenant_id, used, expires_at 
		FROM sso_codes 
		WHERE code = ? FOR UPDATE`

	err = tx.QueryRow(queryCode, code).Scan(
		&sso.ID, &sso.Code, &sso.ClientID, &sso.TenantID, &sso.Used, &sso.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("Mã SSO code không hợp lệ")
	} else if err != nil {
		return nil, fmt.Errorf("Lỗi truy vấn sso_codes: %v", err)
	}

	// 3. Validate logic
	if sso.Used {
		return nil, errors.New("Mã SSO code đã được sử dụng")
	}

	if time.Now().After(sso.ExpiresAt) {
		return nil, errors.New("Mã SSO code đã hết hạn")
	}

	if sso.TenantID != currentTenantID {
		return nil, errors.New("Mã SSO code không thuộc Tenant hiện tại")
	}

	// 4. Đánh dấu code đã sử dụng (used = 1)
	_, err = tx.Exec("UPDATE sso_codes SET used = 1 WHERE id = ?", sso.ID)
	if err != nil {
		return nil, fmt.Errorf("Lỗi cập nhật sso_codes: %v", err)
	}

	// 5. Sinh Plain-text Token & Hashed Token đồng bộ theo chuẩn Sanctum từ utils
	plainTextToken := utils.GenerateRandomString(40)
	hashedToken := utils.HashToken(plainTextToken)

	now := time.Now()
	tokenExpiresAt := now.Add(30 * 24 * time.Hour) // Token có hạn 30 ngày
	tokenableType := "App\\Models\\Central\\Client"
	tokenName := "sso_auth_token"

	// 6. Lưu vào bảng personal_access_tokens của Tenant DB
	insertTokenQuery := `
		INSERT INTO personal_access_tokens 
		(tokenable_type, tokenable_id, name, token, abilities, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	dbTenant, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	res, err := dbTenant.ExecContext(
		c,
		insertTokenQuery,
		tokenableType,
		sso.ClientID,
		tokenName,
		hashedToken,
		`["*"]`, // Full abilities cho Super Admin
		tokenExpiresAt,
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("Lỗi lưu personal_access_tokens: %v", err)
	}

	tokenID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("Lỗi lấy ID token: %v", err)
	}

	// Ghép theo định dạng Sanctum: {id}|{plainTextToken}
	fullSanctumToken := fmt.Sprintf("%d|%s", tokenID, plainTextToken)

	// 7. Lấy thông tin Tenant & User từ Central DB
	var tenantInfo models.SSOTenantInfo
	var user models.SSOUser

	// Query thông tin Client/Tenant
	_ = tx.QueryRow(`
		SELECT tenant_id, IFNULL(subdomain, ''), IFNULL(domain, '') 
		FROM clients WHERE id = ?`, sso.ClientID).Scan(&tenantInfo.TenantID, &tenantInfo.Subdomain, &tenantInfo.Domain)

	if tenantInfo.TenantID == "" {
		tenantInfo.TenantID = sso.TenantID
	}

	// Query thông tin User sở hữu từ Central DB
	_ = tx.QueryRow(`
		SELECT id, email, IFNULL(full_name, ''), is_super_admin 
		FROM users WHERE client_id = ? LIMIT 1`, sso.ClientID).Scan(&user.ID, &user.Email, &user.FullName, &user.IsSuperAdmin)

	// Commit Transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("Lỗi commit transaction: %v", err)
	}

	// 8. Trả về Response
	return &models.SSOExchangeData{
		AccessToken: fullSanctumToken,
		TokenType:   "Bearer",
		ExpiresAt:   tokenExpiresAt.UTC().Format(time.RFC3339),
		TenantInfo:  tenantInfo,
		User:        user,
	}, nil
}
