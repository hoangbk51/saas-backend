package models

import "time"

type TryOnRequest struct {
	ID          uint64     `db:"id" json:"id"`
	ProductID   int        `db:"product_id" json:"product_id"`
	CustomerID  int        `db:"customer_id" json:"customer_id"`
	ProviderID  *string    `db:"provider_id" json:"provider_id"`
	ResultImage *string    `db:"result_image" json:"result_image"`
	Status      *int8      `db:"status" json:"status"`
	ApiResponse *string    `db:"api_response" json:"api_response"`
	CreatedAt   *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at" json:"updated_at"`
}
