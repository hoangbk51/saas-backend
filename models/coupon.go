package models

type Coupon struct {
	ID                  uint32            `db:"id" json:"id"`
	Name                map[string]string `json:"name"`                    // Chuỗi đa ngôn ngữ {"en": "...", "vi": "..."}
	Code                string            `json:"code" binding:"required"` // Mã giảm giá (bắt buộc)
	Description         map[string]string `json:"description"`             // Mô tả đa ngôn ngữ
	Value               float64           `json:"value"`                   // Giá trị giảm (tiền hoặc %)
	MinOrderAmount      float64           `json:"min_order_amount"`        // Giá trị đơn hàng tối thiểu
	Type                string            `json:"type"`                    // 'amount' hoặc 'percent'
	Quantity            *int              `json:"quantity"`                // Tổng số lượng
	QuantityPerCustomer *int              `json:"quantity_per_customer"`   // Giới hạn lượt dùng / khách
	StartingTime        *string           `json:"starting_time"`           // Nhận chuỗi string từ JSON
	EndingTime          *string           `json:"ending_time"`             // Nhận chuỗi string từ JSON
	Active              *bool             `json:"active"`                  // Trang thái bật/tắt (mặc định 1)
	ProductIDs          []int64           `json:"product_ids"`
	CategoryIDs         []int64           `json:"category_ids"`
}
