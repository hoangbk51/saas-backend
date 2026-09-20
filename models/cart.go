package models

import (
	"database/sql"
	"time"
)

/*
	type ShippingMethod struct {
		Label string  `json:"label"`
		Code  string  `json:"code"`
		Value float64 `json:"value"`
	}
*/
type DBShippingMethod struct {
	ID        int64          `db:"id" json:"id"`
	Code      string         `db:"code" json:"code"`
	Name      string         `db:"name" json:"name"`
	Status    int            `db:"status" json:"status"`
	Settings  sql.NullString `db:"settings" json:"settings"` // Lưu JSON config nếu có
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt time.Time      `db:"updated_at" json:"updated_at"`
}

type ShippingMethod struct {
	Code     string            `json:"code"`  // Vd: "ghn", "viettelpost", "flat_rate"
	Label    string            `json:"label"` // Vd: "Giao Hàng Nhanh"
	Value    float64           `json:"value"` // Giá rẻ nhất hoặc mặc định
	Error    string            `json:"error,omitempty"`
	Services []ShippingService `json:"services"` // Danh sách các gói dịch vụ nhỏ bên trong
}

type ShippingService struct {
	Code  string  `json:"code"`  // Vd: "standard", "express"
	Name  string  `json:"name"`  // Vd: "Giao hàng chuẩn", "Giao hàng nhanh 24h"
	Value float64 `json:"value"` // Phí vận chuyển
}

type ZoneResult struct {
	ZoneID int `db:"zone_id"`
}

type RateQueryResult struct {
	Name string  `db:"name"`
	Code string  `db:"code"`
	Rate float64 `db:"rate"`
}

type AddressUpdatePayload struct {
	Shipping struct {
		CountryID  int `json:"country_id"`
		StateID    int `json:"state_id"`
		DistrictID int `json:"district_id"`
		WardID     int `json:"ward_id"`
	} `json:"shipping"`
}

type CartUpdate struct {
	ShippingMethodCode    string `db:"shipping_method_code" json:"shipping_method_code"`
	ShippingMethodSubCode string `db:"shipping_method_sub_code" json:"shipping_method_sub_code"`
	Shipping              struct {
		CountryID  int `json:"country_id"`
		StateID    int `json:"state_id"`
		DistrictID int `json:"district_id"`
		WardID     int `json:"ward_id"`
	} `json:"shipping"`
}

type Cart struct {
	ID                    int64    `db:"id" json:"id"`
	CustomerID            *int64   `db:"customer_id" json:"customer_id,omitempty"`
	IPAddress             *string  `db:"ip_address" json:"ip_address,omitempty"`
	ShippingCountryID     *int64   `db:"shipping_country_id" json:"shipping_country_id,omitempty"`
	ShippingZoneID        *int64   `db:"shipping_zone_id" json:"shipping_zone_id,omitempty"`
	ShippingRateID        *int64   `db:"shipping_rate_id" json:"shipping_rate_id,omitempty"`
	ItemCount             int      `db:"item_count" json:"item_count"`
	Quantity              int      `db:"quantity" json:"quantity"`
	Total                 float64  `db:"total" json:"total"`
	Discount              *float64 `db:"discount" json:"discount,omitempty"`
	Shipping              *float64 `db:"shipping" json:"shipping,omitempty"`
	Taxes                 *float64 `db:"taxes" json:"taxes,omitempty"`
	CouponID              *int64   `db:"coupon_id" json:"coupon_id,omitempty"`
	ShippingMethodCode    *string  `db:"shipping_method_code" json:"shipping_method_code,omitempty"`
	ShippingMethodSubCode *string  `db:"shipping_method_code" json:"shipping_method_sub_code,omitempty"`
	GrandTotal            float64  `db:"grand_total" json:"grand_total"`
	ShippingWeight        float64  `db:"shipping_weight" json:"shipping_weight"`
	BillingAddress        string   `db:"billing_address" json:"billing_address,omitempty"`
	ShippingAddress       string   `db:"shipping_address" json:"shipping_address,omitempty"`
	PaymentMethodID       *int64   `db:"payment_method_id" json:"payment_method_id,omitempty"`
	ShippingStateID       *int64   `db:"shipping_state_id" json:"shipping_state_id,omitempty"`
	ShippingDistrictID    *int64   `db:"shipping_district_id" json:"shipping_district_id,omitempty"`
	ShippingWardID        *int64   `db:"shipping_ward_id" json:"shipping_ward_id,omitempty"`

	CreatedAt *time.Time `db:"created_at" json:"created_at,omitempty"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at,omitempty"`
	Items     []CartItem `db:"-" json:"items,omitempty"`
}

type CartItem struct {
	ID               int64   `json:"id"`
	ProductID        int64   `json:"product_id"`
	ProductVariantID *int64  `json:"product_variant_id"` // Thêm cột này
	ItemDescription  string  `json:"item_description"`
	Quantity         int     `json:"quantity"`
	UnitPrice        float64 `json:"unit_price"`
	Option           string  `json:"option"`
}

type OptionItem struct {
	OptionName  interface{} `json:"option_name"`  // Trả về string hoặc map[string]interface{}
	OptionValue interface{} `json:"option_value"` // Trả về string hoặc map[string]interface{}
}

type CartCharge struct {
	Title string  `json:"title"`
	Value float64 `json:"value"`
	Code  string  `json:"code"`
}

type CartItemResult struct {
	ID               int64        `json:"id"`
	ProductID        int64        `json:"product_id"`
	ProductVariantID *int64       `json:"product_variant_id"` // Trả về int64 hoặc null (không lòi sql.NullInt64)
	ItemDescription  string       `json:"item_description"`   // Khớp với longtext
	Quantity         int          `json:"quantity"`
	UnitPrice        float64      `json:"unit_price"` // Khớp với decimal(20,6)
	Option           []OptionItem `json:"option"`
}

type RecalculateResult struct {
	Count           int              `json:"count"`
	Items           []CartItemResult `json:"items"`
	Total           float64          `json:"total"`
	Charges         []CartCharge     `json:"charges"`
	Data            interface{}      `json:"data"`
	Quantity        int              `json:"quantity"`
	CouponID        int64            `json:"coupon_id"`
	ShippingMethods []ShippingMethod `json:"shipping_methods"` // Bổ sung field này
}

type CartItemWithProduct struct {
	ID               int64         `db:"id"`
	ProductID        int64         `db:"product_id"`
	ProductVariantID sql.NullInt64 `db:"product_variant_id"`
	Quantity         int           `db:"quantity"`
	UnitPrice        float64       `db:"unit_price"`
	ItemDescription  string        `db:"item_description"`
	ProductTitle     string        `db:"title"`
	ProductWeight    float64       `db:"weight"`
	Option           string
	CategoryIDs      []int64 // 🟢 Chuyển thành mảng danh mục
}

type ApplyCouponPayload struct {
	Coupon string `json:"coupon" binding:"required"`
}
