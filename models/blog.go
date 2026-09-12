package models

import (
	"database/sql"
	"time"
)

type Blog struct {
	ID          uint32     `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Excerpt     string     `json:"excerpt"`
	Content     string     `json:"content"`
	UserID      uint64     `json:"user_id"`
	Status      bool       `json:"status"`   // tinyint(1) -> bool
	Approved    bool       `json:"approved"` // tinyint(1) -> bool
	PublishedAt *time.Time `json:"published_at"`
	Likes       uint32     `json:"likes"`
	Dislikes    uint32     `json:"dislikes"`
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
	ModelID     sql.NullInt64  `json:"-"`
	FileName    sql.NullString `json:"-"`
	Image       string         `json:"image"`
	Link        string         `json:"link"`
}
