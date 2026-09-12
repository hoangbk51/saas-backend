package models

// PermissionItem đại diện cho từng checkbox quyền nhỏ (e.g. View, Add, Edit, Delete)
type PermissionItem struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	GuardName   string `json:"guard_name"`
	DisplayName string `json:"display_name"`
}

// ModulePermissionsGroup đại diện cho 1 hàng trên UI (gồm Tên Module + Danh sách quyền đi kèm)
type ModulePermissionsGroup struct {
	ModuleID          uint32           `json:"module_id"`
	ModuleName        string           `json:"module_name"`
	ModuleDescription *string          `json:"module_description"`
	Permissions       []PermissionItem `json:"permissions"`
}
