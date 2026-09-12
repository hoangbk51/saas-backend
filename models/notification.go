package models

import "time"

type Notification struct {
	ID             string  `db:"id" json:"id"`
	Type           string  `db:"type" json:"type"`
	NotifiableType string  `db:"notifiable_type" json:"notifiable_type"`
	NotifiableID   *uint64 `db:"notifiable_id" json:"notifiable_id,omitempty"`

	// --- THÊM 2 TRƯỜNG MỚI NÀY ---
	TargetRole        *string  `db:"target_role" json:"target_role,omitempty"`               // Ví dụ: "ORDER_MANAGER"
	TargetPermissions []string `db:"target_permissions" json:"target_permissions,omitempty"` // Ví dụ: ["orders.read"]
	// ------------------------------

	CreatedBy  *int        `db:"created_by" json:"created_by,omitempty"`
	Icon       *string     `db:"icon" json:"icon,omitempty"`
	ActionText *string     `db:"action_text" json:"action_text,omitempty"`
	ActionURL  *string     `db:"action_url" json:"action_url,omitempty"`
	Message    *string     `db:"message" json:"message,omitempty"`
	Data       interface{} `db:"data" json:"data,omitempty"`
	ReadAt     *time.Time  `db:"read_at" json:"read_at,omitempty"`
	CreatedAt  time.Time   `db:"created_at" json:"created_at"`
}

// Request body khi đánh dấu đã đọc
type MarkReadRequest struct {
	IDs []string `json:"ids"` // Danh sách ID thông báo cần đánh dấu. Nếu rỗng/không truyền -> Đánh dấu ALL
}

// Response trả về danh sách + tổng số thông báo chưa đọc
type NotificationListResponse struct {
	UnreadCount int            `json:"unread_count"`
	Items       []Notification `json:"items"`
}
