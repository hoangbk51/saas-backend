package models

type BlogCategoryInput struct {
	ParentID    *int64  `json:"parent_id"`
	Name        string  `json:"name" binding:"required"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	Active      *bool   `json:"active"`
	Featured    *bool   `json:"featured"`
	Left        *uint8  `json:"left"`
	Right       *uint8  `json:"right"`
}
