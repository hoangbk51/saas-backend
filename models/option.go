package models

type OptionValuePayload struct {
	ID        uint64            `json:"id"`
	Name      map[string]string `json:"name"`
	Image     string            `json:"image"`
	SortOrder int               `json:"sort_order"`
}

type OptionPayload struct {
	Name                 map[string]string    `json:"name" binding:"required"`
	Type                 string               `json:"type"`
	SortOrder            int                  `json:"sort_order"`
	WebsiteID            interface{}          `json:"website_id"`              // Thêm website_id
	Values               []OptionValuePayload `json:"option_values"`           // SỬA: "values" -> "option_values"
	DeleteOptionValueIDs []uint64             `json:"delete_option_value_ids"` // Mảng ID cần xóa
}
