package models

import (
	"encoding/json"
	"time"
)

type PageHistory struct {
	ID int64 `json:"id"`
	//Name        string     `json:"name"`
	Data        json.RawMessage `json:"data"`   // Thay string bằng json.RawMessage	PageThemeID int64      `json:"page_theme_id"`
	Header      json.RawMessage `json:"header"` // Thay string bằng json.RawMessage	PageThemeID int64      `json:"page_theme_id"`
	Footer      json.RawMessage `json:"footer"` // Thay string bằng json.RawMessage	PageThemeID int64      `json:"page_theme_id"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
	PageThemeID int64           `json:"page_theme_id"`
}
