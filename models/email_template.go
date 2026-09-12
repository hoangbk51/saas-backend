package models

import (
	"time"
)

/*
type EmailTemplate struct {
	ID          uint32         `json:"id"`
	Name        string         `json:"name"`
	SenderName  sql.NullString `json:"sender_name"`
	SenderEmail sql.NullString `json:"sender_email"`
	Subject     string         `json:"subject"`
	Body        string         `json:"body"`
	Type        string         `json:"type"` // HTML hoặc Text
	Position    string         `json:"position"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

*/

type EmailTemplate struct {
	ID          uint32     `db:"id" json:"id"`
	Name        string     `db:"name" json:"name"`
	LangCode    string     `db:"lang_code" json:"lang_code"`
	SenderName  *string    `db:"sender_name" json:"sender_name"`
	SenderEmail *string    `db:"sender_email" json:"sender_email"`
	Subject     *string    `db:"subject" json:"subject"`
	Body        *string    `db:"body" json:"body"`
	Type        string     `db:"type" json:"type"`         // 'HTML' | 'Text'
	Position    string     `db:"position" json:"position"` // 'Content' | 'Header' | 'Footer'
	Files       *string    `db:"files" json:"files"`
	CreatedAt   *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

type CreateEmailTemplateReq struct {
	Name        string  `json:"name" binding:"required"`
	LangCode    string  `json:"lang_code"` // Mặc định 'vi' nếu rỗng
	SenderName  *string `json:"sender_name"`
	SenderEmail *string `json:"sender_email"`
	Subject     *string `json:"subject"`
	Body        *string `json:"body"`
	Type        string  `json:"type"`     // HTML hoặc Text
	Position    string  `json:"position"` // Content, Header, Footer
	Files       *string `json:"files"`
}

type UpdateEmailTemplateReq struct {
	Name        string  `json:"name" binding:"required"`
	LangCode    string  `json:"lang_code"`
	SenderName  *string `json:"sender_name"`
	SenderEmail *string `json:"sender_email"`
	Subject     *string `json:"subject"`
	Body        *string `json:"body"`
	Type        string  `json:"type"`
	Position    string  `json:"position"`
	Files       *string `json:"files"`
}
