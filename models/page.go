package models

import "time"

type Page struct {
	ID          uint32     `json:"id"`
	AuthorID    *uint64    `json:"author_id"`
	Title       *string    `json:"title"`
	Slug        *string    `json:"slug"`
	Content     *string    `json:"content"`
	PublishedAt *time.Time `json:"published_at"`
	Visibility  int        `json:"visibility"`
	Position    *string    `json:"position"`
	CreatedAt   *time.Time `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}
