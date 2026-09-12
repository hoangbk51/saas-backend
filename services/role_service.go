package services

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

// 1. GET LIST ROLES (Có phân trang & trả về danh sách permission_ids)
func GetRolesService(c *gin.Context) (interface{}, int, error) {
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
	countQuery := `SELECT COUNT(*) FROM roles`
	err = tenantDB.QueryRowContext(c.Request.Context(), countQuery).Scan(&totalCount)
	if err != nil {
		return gin.H{"error": "Lỗi đếm số lượng roles: " + err.Error()}, http.StatusInternalServerError, err
	}

	roles := make([]map[string]interface{}, 0)
	if totalCount == 0 {
		return utils.BuildLaravelPagination(c, roles, totalCount, page, limit), http.StatusOK, nil
	}

	query := `
		SELECT id, name, guard_name, level, description, created_at, updated_at 
		FROM roles 
		ORDER BY id DESC 
		LIMIT ? OFFSET ?
	`

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, limit, offset)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn roles: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	for rows.Next() {
		item, err := utils.ScanRowToMap(rows)
		if err != nil {
			continue
		}

		// Lấy danh sách permission_ids tương ứng của role
		roleID := item["id"]
		permRows, err := tenantDB.QueryContext(c.Request.Context(), `
			SELECT permission_id FROM role_has_permissions WHERE role_id = ?
		`, roleID)

		permIDs := make([]uint64, 0)
		if err == nil {
			for permRows.Next() {
				var pID uint64
				if err := permRows.Scan(&pID); err == nil {
					permIDs = append(permIDs, pID)
				}
			}
			permRows.Close()
		}

		item["permission_ids"] = permIDs
		roles = append(roles, item)
	}

	return utils.BuildLaravelPagination(c, roles, totalCount, page, limit), http.StatusOK, nil
}

// 2. GET ROLE BY ID
func GetRoleByIDService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")
	query := `SELECT id, name, guard_name, level, description, created_at, updated_at FROM roles WHERE id = ?`

	rows, err := tenantDB.QueryContext(c.Request.Context(), query, id)
	if err != nil {
		return gin.H{"error": "Internal Server Error"}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	if !rows.Next() {
		return gin.H{"error": "Role không tồn tại"}, http.StatusNotFound, nil
	}

	role, err := utils.ScanRowToMap(rows)
	if err != nil {
		return gin.H{"error": "Lỗi parse dữ liệu"}, http.StatusInternalServerError, err
	}

	// Lấy danh sách permission_ids
	permRows, err := tenantDB.QueryContext(c.Request.Context(), `
		SELECT permission_id FROM role_has_permissions WHERE role_id = ?
	`, id)

	permIDs := make([]uint64, 0)
	if err == nil {
		for permRows.Next() {
			var pID uint64
			if err := permRows.Scan(&pID); err == nil {
				permIDs = append(permIDs, pID)
			}
		}
		permRows.Close()
	}
	role["permission_ids"] = permIDs

	return gin.H{"data": role}, http.StatusOK, nil
}

// 3. CREATE ROLE (Transaction)
func CreateRoleService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	var input models.RolePayload
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	if input.GuardName == "" {
		input.GuardName = "web" // Mặc định guard_name là web
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Không thể bắt đầu Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// 1. Thêm Role vào bảng roles
	roleQuery := `
		INSERT INTO roles (name, guard_name, level, description, created_at, updated_at) 
		VALUES (?, ?, ?, ?, NOW(), NOW())
	`
	res, err := tx.ExecContext(c.Request.Context(), roleQuery, input.Name, input.GuardName, input.Level, input.Description)
	if err != nil {
		return gin.H{"error": "Lỗi thêm Role (có thể trùng tên role và guard_name): " + err.Error()}, http.StatusBadRequest, nil
	}

	roleID, _ := res.LastInsertId()

	// 2. Thêm danh sách permissions vào bảng role_has_permissions
	if len(input.PermissionIDs) > 0 {
		placeholders := make([]string, len(input.PermissionIDs))
		args := make([]interface{}, 0, len(input.PermissionIDs)*2)

		for i, permID := range input.PermissionIDs {
			placeholders[i] = "(?, ?)"
			args = append(args, permID, roleID)
		}

		permQuery := fmt.Sprintf(`
			INSERT INTO role_has_permissions (permission_id, role_id) 
			VALUES %s
		`, strings.Join(placeholders, ","))

		_, err = tx.ExecContext(c.Request.Context(), permQuery, args...)
		if err != nil {
			return gin.H{"error": "Lỗi gán Permission cho Role: " + err.Error()}, http.StatusInternalServerError, err
		}
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi Commit Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Tạo vai trò thành công", "id": roleID}, http.StatusCreated, nil
}

// 4. UPDATE ROLE (Transaction)
func UpdateRoleService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	var input models.RolePayload
	if err := c.ShouldBindJSON(&input); err != nil {
		return gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()}, http.StatusBadRequest, nil
	}

	if input.GuardName == "" {
		input.GuardName = "web"
	}

	tx, err := tenantDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		return gin.H{"error": "Không thể bắt đầu Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	// 1. Update bảng roles
	roleQuery := `
		UPDATE roles 
		SET name = ?, guard_name = ?, level = ?, description = ?, updated_at = NOW() 
		WHERE id = ?
	`
	res, err := tx.ExecContext(c.Request.Context(), roleQuery, input.Name, input.GuardName, input.Level, input.Description, id)
	if err != nil {
		return gin.H{"error": "Lỗi cập nhật Role: " + err.Error()}, http.StatusBadRequest, nil
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Role không tồn tại"}, http.StatusNotFound, nil
	}

	// 2. Xóa toàn bộ permissions cũ của role này
	_, err = tx.ExecContext(c.Request.Context(), `DELETE FROM role_has_permissions WHERE role_id = ?`, id)
	if err != nil {
		return gin.H{"error": "Lỗi làm sạch Permissions cũ: " + err.Error()}, http.StatusInternalServerError, err
	}

	// 3. Thêm mới danh sách permissions gửi lên
	if len(input.PermissionIDs) > 0 {
		placeholders := make([]string, len(input.PermissionIDs))
		args := make([]interface{}, 0, len(input.PermissionIDs)*2)

		for i, permID := range input.PermissionIDs {
			placeholders[i] = "(?, ?)"
			args = append(args, permID, id)
		}

		permQuery := fmt.Sprintf(`
			INSERT INTO role_has_permissions (permission_id, role_id) 
			VALUES %s
		`, strings.Join(placeholders, ","))

		_, err = tx.ExecContext(c.Request.Context(), permQuery, args...)
		if err != nil {
			return gin.H{"error": "Lỗi cập nhật Permission cho Role: " + err.Error()}, http.StatusInternalServerError, err
		}
	}

	if err := tx.Commit(); err != nil {
		return gin.H{"error": "Lỗi Commit Transaction: " + err.Error()}, http.StatusInternalServerError, err
	}

	return gin.H{"message": "Cập nhật vai trò thành công"}, http.StatusOK, nil
}

// 5. DELETE ROLE (Khóa ngoại CASCADE sẽ tự xóa trong role_has_permissions)
func DeleteRoleService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	id := c.Param("id")

	res, err := tenantDB.ExecContext(c.Request.Context(), `DELETE FROM roles WHERE id = ?`, id)
	if err != nil {
		return gin.H{"error": "Lỗi xóa Role: " + err.Error()}, http.StatusInternalServerError, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return gin.H{"error": "Role không tồn tại"}, http.StatusNotFound, nil
	}

	return gin.H{"message": "Xóa vai trò thành công"}, http.StatusOK, nil
}
