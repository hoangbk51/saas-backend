package models

// AttributeValue Model đại diện cho bảng attribute_values
type AttributeValue struct {
	ID          uint32            `json:"id"`
	Name        map[string]string `json:"name"` // {"en": "...", "vi": "..."}
	Color       *string           `json:"color"`
	AttributeID uint32            `json:"attribute_id"`
	SortOrder   *int              `json:"sort_order"`
}

// AttributePayload Struct nhận request tạo/sửa Attribute
type AttributePayload struct {
	ID                      uint32            `json:"id"`
	Name                    map[string]string `json:"name"`
	AttributeTypeID         *uint32           `json:"attribute_type_id"`
	Order                   *int              `json:"order"`
	Filterable              *bool             `json:"filterable"` // Nhận true / false chuẩn JSON
	Values                  []AttributeValue  `json:"values"`
	DeleteAttributeValueIDs []uint32          `json:"delete_attribute_value_ids"` // Danh sách ID value cần xóa
}
