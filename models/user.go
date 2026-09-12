package models

// User Model đại diện cho thông tin User trả về
type User struct {
	ID              uint64  `json:"id"`
	Name            *string `json:"name"`
	NiceName        *string `json:"nice_name"`
	Email           string  `json:"email"`
	DOB             *string `json:"dob"`
	Sex             *string `json:"sex"`
	Description     *string `json:"description"`
	Active          bool    `json:"active"`
	LastVisitedAt   *string `json:"last_visited_at,omitempty"`
	LastVisitedFrom *string `json:"last_visited_from,omitempty"`
	EmailVerifiedAt *string `json:"email_verified_at,omitempty"`
	RoleID          *uint64 `json:"role_id,omitempty"`
	RoleName        *string `json:"role_name,omitempty"`
	CreatedAt       *string `json:"created_at,omitempty"`
	UpdatedAt       *string `json:"updated_at,omitempty"`
}

// CreateUserPayload Struct nhận request tạo User
type CreateUserPayload struct {
	Name        *string `json:"name"`
	NiceName    *string `json:"nice_name"`
	Email       string  `json:"email" binding:"required,email"`
	Password    string  `json:"password" binding:"required,min=6"`
	DOB         *string `json:"dob"` // YYYY-MM-DD
	Sex         *string `json:"sex"` // male, female, other
	Description *string `json:"description"`
	Active      *bool   `json:"active"` // Default true
	RoleID      uint64  `json:"role_id" binding:"required"`
}

// UpdateUserPayload Struct nhận request cập nhật User
type UpdateUserPayload struct {
	Name        *string `json:"name"`
	NiceName    *string `json:"nice_name"`
	Email       string  `json:"email" binding:"required,email"`
	Password    *string `json:"password"` // Không bắt buộc, nếu gửi lên mới update password
	DOB         *string `json:"dob"`
	Sex         *string `json:"sex"`
	Description *string `json:"description"`
	Active      *bool   `json:"active"`
	RoleID      uint64  `json:"role_id" binding:"required"`
}

type UserLoginResponse struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	Password     string   `json:"-"` // Không trả về password trong JSON
	OrderCount   int      `json:"orders_count"`
	Avatar       string   `json:"avatar"`
	FinalToken   string   `json:"-"`
	IsSuperAdmin bool     `json:"is_super_admin"`
	Permissions  []string `json:"permissions"` // Danh sách plugin codes cho client phân quyền
}

// Cấu trúc nhận dữ liệu từ Client
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}
