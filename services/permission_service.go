package services

import (
	"net/http"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func GetGroupedPermissionsService(c *gin.Context) (interface{}, int, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return gin.H{"error": err.Error()}, http.StatusInternalServerError, err
	}

	// SQL Query Lấy Module và Permissions liên quan
	query := `
		SELECT 
			m.id AS module_id,
			m.name AS module_name,
			m.description AS module_description,
			p.id AS permission_id,
			p.name AS permission_name,
			p.guard_name,
			p.display_name
		FROM modules m
		LEFT JOIN permissions p ON p.module_id = m.id
		WHERE m.deleted_at IS NULL AND m.active = 1
		ORDER BY m.name ASC, p.id ASC
	`

	rows, err := tenantDB.QueryContext(c.Request.Context(), query)
	if err != nil {
		return gin.H{"error": "Lỗi truy vấn danh sách permissions: " + err.Error()}, http.StatusInternalServerError, err
	}
	defer rows.Close()

	// Dùng map để group permissions theo module_id giữ nguyên thứ tự
	moduleMap := make(map[uint32]*models.ModulePermissionsGroup)
	moduleOrders := make([]uint32, 0) // Giữ thứ tự xuất hiện của Module

	for rows.Next() {
		var (
			moduleID       uint32
			moduleName     string
			moduleDesc     *string
			permissionID   *uint64
			permissionName *string
			guardName      *string
			displayName    *string
		)

		err := rows.Scan(
			&moduleID,
			&moduleName,
			&moduleDesc,
			&permissionID,
			&permissionName,
			&guardName,
			&displayName,
		)
		if err != nil {
			return gin.H{"error": "Lỗi scan dữ liệu: " + err.Error()}, http.StatusInternalServerError, err
		}

		// Nếu chưa có module trong map thì khởi tạo
		if _, exists := moduleMap[moduleID]; !exists {
			moduleMap[moduleID] = &models.ModulePermissionsGroup{
				ModuleID:          moduleID,
				ModuleName:        moduleName,
				ModuleDescription: moduleDesc,
				Permissions:       make([]models.PermissionItem, 0),
			}
			moduleOrders = append(moduleOrders, moduleID)
		}

		// Nếu có permission tương ứng với module thì append vào danh sách permissions của module đó
		if permissionID != nil {
			item := models.PermissionItem{
				ID:          *permissionID,
				Name:        *permissionName,
				GuardName:   utils.GetStringValue(guardName),
				DisplayName: utils.GetStringValue(displayName),
			}

			// Ưu tiên dùng display_name nếu có, nếu không thì fallback về name
			if item.DisplayName == "" {
				item.DisplayName = item.Name
			}

			moduleMap[moduleID].Permissions = append(moduleMap[moduleID].Permissions, item)
		}
	}

	// Chuyển từ map sang slice để trả về JSON
	result := make([]models.ModulePermissionsGroup, 0, len(moduleOrders))
	for _, mID := range moduleOrders {
		result = append(result, *moduleMap[mID])
	}

	return gin.H{"data": result}, http.StatusOK, nil
}
