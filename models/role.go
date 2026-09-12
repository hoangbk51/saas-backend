package models

// Role Model đại diện cho bảng roles
type Role struct {
	ID          uint64   `json:"id"`
	Name        string   `json:"name" binding:"required"`
	GuardName   string   `json:"guard_name"`
	Level       *int     `json:"level"`
	Description *string  `json:"description"`
	Permissions []uint64 `json:"permissions,omitempty"` // Danh sách ID permission đi kèm
	CreatedAt   *string  `json:"created_at,omitempty"`
	UpdatedAt   *string  `json:"updated_at,omitempty"`
}

// RolePayload Struct nhận Request Tạo / Sửa Role
type RolePayload struct {
	Name          string   `json:"name" binding:"required"`
	GuardName     string   `json:"guard_name"`
	Level         *int     `json:"level"`
	Description   *string  `json:"description"`
	PermissionIDs []uint64 `json:"permission_ids"` // Danh sách Permission IDs gán cho Role
}
