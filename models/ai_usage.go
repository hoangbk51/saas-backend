package models

import "time"

type AIUsage struct {
	ID               int64     `json:"id"`
	TenantID         int64     `json:"tenant_id"`
	UserID           int64     `json:"user_id"`
	Type             string    `json:"type"` // e.g., "website-block-clone"
	ModelName        string    `json:"model_name"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	ReferenceID      int64     `json:"reference_id"`
	CreatedAt        time.Time `json:"created_at"`
}
