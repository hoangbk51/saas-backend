package models

import "time"

// Domain đại diện cho bảng domains
type Domain struct {
	ID        uint      `json:"id"`
	Domain    string    `json:"domain"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AddDomainRequest nhận dữ liệu truyền vào từ Client
type AddDomainRequest struct {
	Domain string `json:"domain" binding:"required"`
}
