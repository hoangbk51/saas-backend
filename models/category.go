package models

import (
	"time"
)

type Category struct {
	ID          uint32      `db:"id" json:"id"`
	ParentID    *uint32     `db:"parent_id" json:"parent_id"`
	Name        string      `db:"name" json:"name"`
	Slug        string      `db:"slug" json:"slug"`
	Description string      `db:"description" json:"description"`
	Active      bool        `db:"active" json:"active"`
	Featured    bool        `db:"featured" json:"featured"`
	CreatedAt   time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time   `db:"updated_at" json:"updated_at"`
	Children    []*Category `json:"children"` // Không cần tag db vì đây là trường quan hệ động
}

type CategoryPayload struct {
	ParentID    uint32            `json:"parent_id"`
	Name        map[string]string `json:"name" binding:"required"`
	Description map[string]string `json:"description" `
	Active      bool              `json:"active"`
	Featured    bool              `json:"featured"`
}
