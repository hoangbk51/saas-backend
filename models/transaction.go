package models

import "time"

type Transaction struct {
	ID            int64      `json:"id" db:"id"`
	OrderID       int64      `json:"order_id" db:"order_id"`
	ReferenceID   string     `json:"reference_id" db:"reference_id"`
	PaymentMethod string     `json:"payment_method" db:"payment_method"`
	Status        int        `json:"status" db:"status"`
	Code          string     `json:"code" db:"code"`
	Amount        float64    `json:"amount" db:"amount"`
	Type          int        `json:"type" db:"type"`
	CustomerID    int        `json:"customer_id" db:"customer_id"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at" db:"deleted_at"`
}
