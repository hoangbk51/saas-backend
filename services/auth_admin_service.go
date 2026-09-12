package services

import (
	"fmt"
	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func AdminLoginHandler(c *gin.Context) (*models.UserLoginResponse, error) {
	user := &models.UserLoginResponse{}
	var req models.LoginRequest

	// 1. Validate dữ liệu đầu vào
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, fmt.Errorf("Email và Password là bắt buộc")
	}

	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể lấy kết nối database tenant: %v", err)
	}

	var tokenableType string
	var isSuperAdmin bool

	// ==================================================================
	// TẦNG 1: Kiểm tra CẢ Email VÀ Password trong TENANT DB
	// ==================================================================
	tenantQuery := `SELECT id, name, email, password FROM users WHERE email = ? LIMIT 1`
	utils.LogSQL(tenantQuery, req.Email)

	errTenant := tenantDB.QueryRowContext(c, tenantQuery, req.Email).Scan(
		&user.ID, &user.Name, &user.Email, &user.Password,
	)

	// Điều kiện Tầng 1: Tìm thấy User VÀ Khớp Password ở Tenant DB
	if errTenant == nil && bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) == nil {

		// ĐĂNG NHẬP TENANT THÀNH CÔNG -> Bỏ qua Tầng 2 (Central DB)
		tokenableType = "App\\Models\\Tenant\\User"
		isSuperAdmin = false

	} else {

		// ==================================================================
		// TẦNG 2: Nếu Tầng 1 thất bại (Không có email HOẶC sai pass Tenant)
		// ==================================================================
		centralDB, errCentralDB := utils.GetCentralDB()
		if errCentralDB != nil {
			return nil, fmt.Errorf("không thể kết nối central database: %v", errCentralDB)
		}

		centralQuery := `SELECT id, IFNULL(full_name, email), email, password FROM clients WHERE email = ? LIMIT 1`
		utils.LogSQL(centralQuery, req.Email)

		errCentral := centralDB.QueryRowContext(c, centralQuery, req.Email).Scan(
			&user.ID, &user.Name, &user.Email, &user.Password,
		)

		// Nếu ở Central DB không thấy user HOẶC mật khẩu sai -> Báo lỗi Unauthorized
		if errCentral != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
			return nil, fmt.Errorf("Unauthorized")
		}

		// ĐĂNG NHẬP CENTRAL (SUPER ADMIN) THÀNH CÔNG
		tokenableType = "App\\Models\\Central\\Client"
		isSuperAdmin = true
	}

	// Gán cờ is_super_admin vào response model
	user.IsSuperAdmin = isSuperAdmin

	// ==================================================================
	// BƯỚC TIẾP THEO: Tạo Sanctum Personal Access Token & Lưu vào TENANT DB
	// ==================================================================
	plainToken := utils.GenerateRandomString(20)
	hashedToken := utils.HashToken(plainToken)

	insertQuery := `
		INSERT INTO personal_access_tokens 
		(tokenable_type, tokenable_id, name, token, abilities, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())`

	insertArgs := []interface{}{
		tokenableType,
		user.ID,
		"authToken",
		hashedToken,
		"[\"*\"]",
	}
	utils.LogSQL(insertQuery, insertArgs...)

	res, err := tenantDB.ExecContext(c, insertQuery, insertArgs...)
	if err != nil {
		return nil, fmt.Errorf("Error creating token: %v", err)
	}

	lastID, _ := res.LastInsertId()
	user.FinalToken = fmt.Sprintf("%d|%s", lastID, plainToken)

	var currentDB string
	_ = tenantDB.QueryRowContext(c, "SELECT DATABASE()").Scan(&currentDB)
	utils.LogToFile("[DEBUG] Target User ID: %v | Type: %s | Active DB: %s", user.ID, tokenableType, currentDB)

	// ==================================================================
	// BƯỚC PHÂN QUYỀN (PERMISSIONS)
	// ==================================================================
	if isSuperAdmin {
		user.Permissions = []string{"*"}
		utils.LogToFile("[RESULT] SuperAdmin logged in via Central Client ID %d with full permissions", user.ID)
	} else {
		permQuery := fmt.Sprintf(`
			SELECT DISTINCT p.name 
			FROM permissions p
			INNER JOIN role_has_permissions rhp ON p.id = rhp.permission_id
			INNER JOIN model_has_roles mhr ON rhp.role_id = mhr.role_id
			WHERE mhr.model_id = %d`, user.ID)

		utils.LogSQL(permQuery)

		rows, err := tenantDB.QueryContext(c, permQuery)
		if err != nil {
			utils.LogToFile("[Permission Error] QueryContext failed: %v\n", err)
		} else {
			defer rows.Close()

			permissions := make([]string, 0)
			for rows.Next() {
				var permName string
				if err := rows.Scan(&permName); err != nil {
					utils.LogToFile("[Permission Error] Scan error for userID %d: %v\n", user.ID, err)
					continue
				}
				permissions = append(permissions, permName)
			}

			if err := rows.Err(); err != nil {
				utils.LogToFile("[Permission Error] Rows iteration error: %v\n", err)
			}

			utils.LogToFile("[RESULT] Found %d permissions for userID %d", len(permissions), user.ID)
			user.Permissions = permissions
		}
	}

	return user, nil
}
