package models

import (
	"time"
)

type Order struct {
	ID                int64   `db:"id"`
	OrderNumber       string  `db:"order_number"`
	GrandTotal        float64 `db:"grand_total"`
	CustomerID        *int64  `db:"customer_id"`
	PaymentMethodCode string  `db:"payment_method_code"`
	CouponID          *int64  `db:"coupon_id"`
	Email             string  `db:"email"`
	CustomerPhone     string  `db:"customer_phone"`
	BillingFirstName  string  `db:"billing_first_name"`
	BillingLastName   string  `db:"billing_last_name"`
	BillingAddress1   string  `db:"billing_address1" json:"billing_address_1"`
	BillingAddress2   string  `db:"billing_address1" json:"billing_address_2"`
	BillingCity       *int64  `db:"billing_city"` //todo district
	BillingState      *int64  `db:"billing_state"`
	BillingCountry    *int64  `db:"billing_country"`
	BillingZip        string  `db:"billing_zip"`

	ShippingFirstName string `db:"shipping_first_name"`
	ShippingLastName  string `db:"shipping_last_name"`
	ShippingAddress1  string `db:"shipping_address1" json:"shipping_address_1"`
	ShippingAddress2  string `db:"shipping_address1" json:"shipping_address_2"`

	ShippingCity    *int64 `db:"shipping_city"`
	ShippingZip     string `db:"shipping_zip"`
	ShippingState   *int64 `db:"shipping_state"`
	ShippingCountry *int64 `db:"shipping_country"`
	ShippingWard    *int64 `db:"shipping_ward"`

	TotalWeight           float64   `db:"total_weight"`
	CreatedAt             time.Time `json:"created_at"`
	ShippingMethodSubCode string    `db:"shipping_method_sub_code"`
	ShippingMethodCode    string    `db:"shipping_method_code"`
	OrderStatusID         int64     `db:"order_status_id"`
}

type OrderRequest struct {
	Billing       map[string]interface{} `json:"billing"`
	Shipping      map[string]interface{} `json:"shipping"`
	OtherAddress  bool                   `json:"otherAddress"`
	CustomerEmail string                 `json:"customer_email"`
	CustomerPhone string                 `json:"customer_phone"`
	CustomerID    *int64                 `json:"customer_id"`
	Note          string                 `json:"note"`
	PaymentMethod string                 `json:"payment_method"`
}

type OrderItem struct {
	ID               int64        `json:"id" db:"id"`
	ProductID        int64        `json:"product_id" db:"product_id"`
	ProductVariantID *int64       `json:"product_variant_id,omitempty" db:"product_variant_id"`
	ItemDescription  string       `json:"item_description" db:"item_description"`
	Option           []OptionItem `json:"option" db:"option"`
	Quantity         int          `json:"quantity" db:"quantity"`
	UnitPrice        float64      `json:"unit_price" db:"unit_price"`
}

type OrderTotal struct {
	Title string  `db:"title"`
	Value float64 `db:"value"`
}

// OrderFilterParam chứa tất cả tham số search, filter và order
type OrderFilterParam struct {
	ID             string   `form:"id"`
	CustomerName   string   `form:"customer_name"`
	Email          string   `form:"email"`
	TotalMin       *float64 `form:"total_min"`
	TotalMax       *float64 `form:"total_max"`
	FromDate       string   `form:"from_date"` // Định dạng: YYYY-MM-DD
	ToDate         string   `form:"to_date"`   // Định dạng: YYYY-MM-DD
	PaymentMethod  string   `form:"payment_method"`
	OrderStatus    string   `form:"order_status"`
	ShippingMethod string   `form:"shipping_method"`

	// Pagination & Sorting
	SortBy    string `form:"sort_by"`    // id, grand_total, created_at, customer_name...
	SortOrder string `form:"sort_order"` // asc hoặc desc
	Page      int    `form:"page"`
	Limit     int    `form:"limit"`
}

// OrderListResponse struct trả về thông tin danh sách kèm phân trang
type OrderListResponse struct {
	Items       []Order `json:"items"`
	TotalItems  int64   `json:"total_items"`
	TotalPages  int     `json:"total_pages"`
	CurrentPage int     `json:"current_page"`
	Limit       int     `json:"limit"`
}

type OrderHistory struct {
	ID            int64  `json:"id"`
	OrderID       int64  `json:"order_id"`
	OrderStatusID int64  `json:"order_status_id"`
	Comment       string `json:"comment"`
	Notify        int    `json:"notify"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type OrderDetailResponse struct {
	ID                 int64   `json:"id"`
	OrderNumber        string  `json:"order_number"`
	GrandTotal         float64 `json:"grand_total"`
	CustomerID         *int64  `json:"customer_id"`
	PaymentMethodCode  string  `json:"payment_method_code"`
	ShippingMethodCode string  `json:"shipping_method_code"`
	OrderStatusId      int64   `json:"order_status_id"`
	CouponID           *int64  `json:"coupon_id"`
	Email              string  `json:"email"`
	CustomerPhone      string  `json:"customer_phone"`
	BillingFirstName   string  `json:"billing_first_name"`
	BillingLastName    string  `json:"billing_last_name"`
	BillingAddress1    string  `json:"billing_address1"`
	BillingAddress2    string  `json:"billing_address2"`
	BillingCity        *int64  `json:"billing_city"`
	BillingState       *int64  `json:"billing_state"`
	BillingCountry     *int64  `json:"billing_country"`
	BillingWard        *int64  `json:"billing_ward"`

	BillingZip        string         `json:"billing_zip"`
	ShippingFirstName string         `json:"shipping_first_name"`
	ShippingLastName  string         `json:"shipping_last_name"`
	ShippingAddress1  string         `json:"shipping_address1"`
	ShippingAddress2  string         `json:"shipping_address2"`
	ShippingState     *int64         `json:"shipping_state"`
	ShippingCountry   *int64         `json:"shipping_country"`
	ShippingCity      *int64         `json:"shipping_city"`
	ShippingWard      *int64         `json:"shipping_ward"`
	ShippingZip       string         `json:"shipping_zip"`
	Items             []OrderItem    `json:"items"`
	Totals            []OrderTotal   `json:"totals"`
	Histories         []OrderHistory `json:"histories"` // 👈 THÊM DÒNG NÀY
	TrackingCode      *string
	TotalWeight       float64 `json:"total_weight" db:"total_weight"`
}

// Struct nhận JSON payload từ Frontend
type UpdateOrderRequest struct {
	ID                 int64   `json:"id"`
	OrderNumber        string  `json:"order_number"`
	CustomerID         *int64  `json:"customer_id"`
	Email              string  `json:"email"`
	CustomerPhone      string  `json:"customer_phone"`
	Note               string  `json:"note"`
	PaymentMethodCode  string  `json:"payment_method_code"`
	PaymentMethod      string  `json:"paymentMethod"`
	ShippingMethodCode string  `json:"shipping_method_code"`
	Status             int64   `json:"status"`
	Total              float64 `json:"total"`

	// Billing fields
	BillingFirstName string `json:"billing_first_name"`
	BillingLastName  string `json:"billing_last_name"`
	BillingAddress1  string `json:"billing_address1"`
	BillingCity      string `json:"billing_city"`
	BillingState     string `json:"billing_state"`
	BillingZip       string `json:"billing_zip"`
	BillingCountry   string `json:"billing_country"`

	// Shipping fields
	ShippingFirstName string `json:"shipping_first_name"`
	ShippingLastName  string `json:"shipping_last_name"`
	ShippingAddress1  string `json:"shipping_address1"`
	ShippingCity      string `json:"shipping_city"`
	ShippingState     string `json:"shipping_state"`
	ShippingZip       string `json:"shipping_zip"`
	ShippingCountry   string `json:"shipping_country"`

	Items []struct {
		ID              int64   `json:"id"`
		Name            string  `json:"name"`
		Price           float64 `json:"price"`
		UnitPrice       float64 `json:"unit_price"`
		ItemDescription string  `json:"item_description"`
		Quantity        int     `json:"quantity"`
	} `json:"items"`
}

// Giữ lại DTO cho Service layer
type UpdateOrderDTO struct {
	OrderID            int64
	Email              string
	CustomerPhone      string
	Note               string
	PaymentMethod      string
	ShippingMethodCode string
	Status             int64

	// Tách riêng Billing & Shipping
	BillingFirstName string
	BillingLastName  string
	BillingAddress1  string
	BillingCity      string
	BillingState     string
	BillingZip       string
	BillingCountry   string

	ShippingFirstName string
	ShippingLastName  string
	ShippingAddress1  string
	ShippingCity      string
	ShippingState     string
	ShippingZip       string
	ShippingCountry   string

	Items []CartItem
}
