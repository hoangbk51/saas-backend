package services

import (
	"net/http"
	"strconv"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const ModelTypeUser = "App\\Models\\User" // Giá trị mặc định của model_type

// 1. GET LIST USERS (Phân trang + JOIN lấy Role)
func GetUsersService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	var totalCount int
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery).Scan(&totalCount)
	if err != nil {
		return gin.H{"error": "Lỗi đếm số lượng users: " + err.Error()}, http.StatusInternalServerError, err
	}

	users := make([]map[string]interface{}, 0)
	if totalCount == 0 {
		return utils.BuildLaravelPagination(c, users, totalCount, page, limit), http.StatusOK, nil
	}

	query := `
		SELECT 
			u.id, u.name, u.nice_name, u.email, u.dob, u.sex, u.description, 
			u.active, u.last_visited_at, u.last_visited_from, u.created_at, u.updated_at,
			mhr.role_id, r.name AS role_name
		FROM users u
		LEFT JOIN model_has_roles mhr ON mhr.model_id = u.id AND mhr.model_type = ?
		LEFT JOIN roles r ON r.id = mhr.role_id
		WHERE u.deleted_at IS NULL
		ORDER BY u.id DESC
		LIMIT ? OFFSET ?
	`

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, ModelTypeUser, limit, offset)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn users: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := utils.ScanRowToMap(rows)
		if err == nil {
			users = append(users, item)
		}
	}

	return utils.BuildLaravelPagination(c, users, totalCount, page, limit), http.StatusOK, nil
}

// 2. GET USER BY ID
func GetUserByIDService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")
	query := `
		SELECT 
			u.id, u.name, u.nice_name, u.email, u.dob, u.sex, u.description, 
			u.active, u.last_visited_at, u.last_visited_from, u.created_at, u.updated_at,
			mhr.role_id, r.name AS role_name
		FROM users u
		LEFT JOIN model_has_roles mhr ON mhr.model_id = u.id AND mhr.model_type = ?
		LEFT JOIN roles r ON r.id = mhr.role_id
		WHERE u.id = ? AND u.deleted_at IS NULL
	`

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, ModelTypeUser, id)
	if err != nil {
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Người dùng không tồn tại"}, http.StatusNotFound, nil
	}

	user, err := utils.ScanRowToMap(rows)
	if err != nil {
		return gin.H{"error": "Lỗi parse dữ liệu"}, http.StatusInternalServerError, err
	}

	return gin.H{"data": user}, http.StatusOK, nil
}

// 3. CREATE USER (Transaction)
func CreateUserService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.CreateUserPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	// Hash Password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return gin.H{"error": "Lỗi mã hóa mật khẩu"}, http.StatusInternalServerError, err
	}

	activeVal := true
	if input.Active != nil {
		activeVal = *input.Active
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Không thể bắt đầu Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// 1. Thêm User vào bảng users
	userQuery := `
		INSERT INTO users (name, nice_name, email, password, dob, sex, description, active, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`
	res, err := tx.ExecContext(c.Request.Context(), userQuery,
		input.Name, input.NiceName, input.Email, string(hashedPassword),
		input.DOB, input.Sex, input.Description, activeVal,
	)
	if err != nil {
		return gin.H{"error": "Lỗi thêm User (Email có thể đã tồn tại): " + err.Error()}, http.StatusBadRequest, nil
	}

	userID, _ := res.LastInsertId()

	// 2. Thêm Role vào bảng model_has_roles
	roleQuery := `
		INSERT INTO model_has_roles (role_id, model_type, model_id) 
		VALUES (?, ?, ?)
	`
	_, err = tx.ExecContext(c.Request.Context(), roleQuery, input.RoleID, ModelTypeUser, userID)
	if err != nil {
		return gin.H{"error": "Lỗi gán Role cho User (Role ID không tồn tại): " + err.Error()}, http.StatusBadRequest, nil
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi Commit Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Tạo người dùng thành công", "id": userID}, http.StatusCreated, nil
}

// 4. UPDATE USER (Transaction)
func UpdateUserService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	var input models.UpdateUserPayload
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Không thể bắt đầu Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// 1. Cập nhật bảng users (xử lý tùy chọn có đổi password hay không)
	var userQuery string
	var args []interface{}

	if input.Password != nil && *input.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return gin.H{"error": "Lỗi mã hóa mật khẩu"}, http.StatusInternalServerError, err
		}

		userQuery = `
			UPDATE users 
			SET name = ?, nice_name = ?, email = ?, password = ?, dob = ?, sex = ?, description = ?, active = COALESCE(?, active), updated_at = NOW() 
			WHERE id = ? AND deleted_at IS NULL
		`
		args = []interface{}{input.Name, input.NiceName, input.Email, string(hashedPassword), input.DOB, input.Sex, input.Description, input.Active, id}
	} else {
		userQuery = `
			UPDATE users 
			SET name = ?, nice_name = ?, email = ?, dob = ?, sex = ?, description = ?, active = COALESCE(?, active), updated_at = NOW() 
			WHERE id = ? AND deleted_at IS NULL
		`
		args = []interface{}{input.Name, input.NiceName, input.Email, input.DOB, input.Sex, input.Description, input.Active, id}
	}

	res, err := tx.ExecContext(c.Request.Context(), userQuery, args...)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật User: " + err.Error()}, http.StatusBadRequest, nil
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Người dùng không tồn tại hoặc đã bị xóa"}, http.StatusNotFound, nil
	}

	// 2. Xóa role cũ và thêm role mới vào model_has_roles
	_, err = tx.ExecContext(c.Request.Context(), `
		DELETE FROM model_has_roles WHERE model_id = ? AND model_type = ?
	`, id, ModelTypeUser)
	if err != nil {
		return gin.H{"error": "Lỗi làm sạch Role cũ: " + err.Error()}, http.StatusInternalServerError, err
	}

	roleQuery := `
		INSERT INTO model_has_roles (role_id, model_type, model_id) 
		VALUES (?, ?, ?)
	`
	_, err = tx.ExecContext(c.Request.Context(), roleQuery, input.RoleID, ModelTypeUser, id)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật Role cho User: " + err.Error()}, http.StatusBadRequest, nil
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi Commit Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Cập nhật người dùng thành công"}, http.StatusOK, nil
}

// 5. DELETE USER (Soft delete)
func DeleteUserService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	res, err := tenantDB.ExecContext(c.Request.Context(), `
		UPDATE users SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL
	`, id)
	if err != nil {
		return gin.H{"error": "Lỗi xóa User: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Người dùng không tồn tại hoặc đã bị xóa trước đó"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa người dùng thành công"}, http.StatusOK, nil
}
