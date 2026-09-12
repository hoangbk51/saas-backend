package models

import "encoding/json"

type TenantInfoResponse struct {
	TenantID string          `json:"tenant_id"`
	Domain   string          `json:"domain"`
	Details  json.RawMessage `json:"details"` // Hứng chuỗi JSON từ cột data và giữ nguyên cấu trúc Object
}
