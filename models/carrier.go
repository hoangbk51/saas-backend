package models

import "time"

type Carrier struct {
	ID          uint32     `db:"id" json:"id"`
	TaxID       *uint32    `db:"tax_id" json:"tax_id"`
	Name        string     `db:"name" json:"name"`
	Code        *string    `db:"code" json:"code"`
	Email       *string    `db:"email" json:"email"`
	Phone       *string    `db:"phone" json:"phone"`
	TrackingURL *string    `db:"tracking_url" json:"tracking_url"`
	Active      bool       `db:"active" json:"active"`
	Setting     *string    `db:"setting" json:"setting"`
	Description *string    `db:"description" json:"description"`
	CreatedAt   *time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   *time.Time `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

type CreateCarrierReq struct {
	TaxID       *uint32 `json:"tax_id"`
	Name        string  `json:"name" binding:"required"`
	Code        *string `json:"code"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	TrackingURL *string `json:"tracking_url"`
	Active      *bool   `json:"active"` // Mặc định true nếu nil
	Setting     *string `json:"setting"`
	Description *string `json:"description"`
}

type UpdateCarrierReq struct {
	TaxID       *uint32 `json:"tax_id"`
	Name        string  `json:"name" binding:"required"`
	Code        *string `json:"code"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	TrackingURL *string `json:"tracking_url"`
	Active      bool    `json:"active"`
	Setting     *string `json:"setting"`
	Description *string `json:"description"`
}

type SaveCarrierRateTableReq struct {
	Conditions []ConditionReq                    `json:"conditions"`
	Rates      map[string]map[string]interface{} `json:"rates"` // rates[zone_id][condition_id] = rate
}

type ConditionReq struct {
	ID        int         `json:"id"` // 0 hoặc nil nếu là condition mới
	CarrierID int         `json:"carrier_id"`
	Min       interface{} `json:"min"`
	Max       interface{} `json:"max"`
	SortOrder int         `json:"sort_order"`
}
