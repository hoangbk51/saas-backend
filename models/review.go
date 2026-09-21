package models

import "time"

type Review struct {
	ID         uint64     `db:"id" json:"id"`
	CustomerID uint64     `db:"customer_id" json:"customer_id"`
	ProductID  uint64     `db:"product_id" json:"product_id"`
	Rating     *int8      `db:"rating" json:"rating"`
	Comment    *string    `db:"comment" json:"comment"`
	Approved   bool       `db:"approved" json:"approved"`
	Spam       bool       `db:"spam" json:"spam"`
	CreatedAt  *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at" json:"updated_at"`
}

type ReviewInput struct {
	ProductID  int    `json:"product_id" binding:"required"`
	Comment    string `json:"comment" binding:"required"`
	Rating     int    `json:"rating" binding:"required,min=1,max=5"`
	CustomerID int    `json:"customer_id"`
}

type ReviewUpdateInput struct {
	Rating   *int8   `json:"rating" binding:"omitempty,min=1,max=5"`
	Comment  *string `json:"comment"`
	Approved *bool   `json:"approved"`
	Spam     *bool   `json:"spam"`
}
