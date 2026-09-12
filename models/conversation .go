package models

import "time"

type Conversation struct {
	ID              int64      `json:"id" db:"id"`
	CustomerID      int64      `json:"customer_id" db:"customer_id"`
	AssignedAdminID *int64     `json:"assigned_admin_id" db:"assigned_admin_id"`
	Status          string     `json:"status" db:"status"`
	LastMessageAt   *time.Time `json:"last_message_at" db:"last_message_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	Channel         string     `json:"channel" db:"channel"`
	ExternalChatId  *string    `json:"external_chat_id" db:"external_chat_id"`
	CustomerName    *string    `json:"customer_name" db:"customer_name"`
}

type CreateConversationResponse struct {
	Conversation Conversation `json:"conversation"`
	GuestToken   string       `json:"guest_token,omitempty"`
}
