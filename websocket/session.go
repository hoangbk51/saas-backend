package websocket

import "github.com/jmoiron/sqlx"

type ChatSession struct {
	TenantID       string
	ConversationID int64

	CustomerID int64

	AdminID      int64
	IsSuperAdmin bool

	Role string

	DB *sqlx.DB
}
