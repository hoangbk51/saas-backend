package models

import (
	"database/sql"
	"time"
)

type Product struct {
	ID               uint64            `json:"id" form:"id"`
	Title            map[string]string `json:"title" form:"title"`                         // {"en": "...", "vi": "..."}
	Description      map[string]string `json:"description" form:"description"`             // {"en": "...", "vi": "..."}
	ShortDescription map[string]string `json:"short_description" form:"short_description"` // {"en": "...", "vi": "..."}
	MetaTitle        map[string]string `json:"meta_title" form:"meta_title"`               // {"en": "...", "vi": "..."}
	MetaDescription  map[string]string `json:"meta_description" form:"meta_description"`   // {"en": "...", "vi": "..."}

	BrandID        int64   `json:"brand_id" form:"brand_id"`
	ModelNumber    string  `json:"model_number" form:"model_number"`
	SKU            string  `json:"sku" form:"sku"`
	Condition      string  `json:"condition" form:"condition"`
	StockQuantity  int     `json:"stock_quantity" form:"stock_quantity"`
	PurchasePrice  float64 `json:"purchase_price" form:"purchase_price"`
	SalePrice      float64 `json:"price" form:"price"`
	ShippingWeight float64 `json:"shipping_weight" form:"shipping_weight"`
	Active         bool    `json:"active" form:"active"`
	Slug           string  `json:"slug" form:"slug"`

	LinkVideo          string    `json:"link_video" form:"link_video"`
	CreatedAt          time.Time `json:"created_at" form:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" form:"updated_at"`
	OfferPrice         float64   `json:"offer_price" form:"offer_price"`
	FinalPrice         float64   `json:"final_price" form:"final_price"`
	HasOffer           bool      `json:"has_offer" form:"has_offer"`
	DiscountPercentage float64   `json:"discount_percentage" form:"discount_percentage"`
	OfferStartStr      string    `json:"offer_start" form:"offer_start"`
	OfferEndStr        string    `json:"offer_end" form:"offer_end"`
	TryOnStatus        int64     `json:"try_on_status" form:"try_on_status"`
	Rate               float64   `json:"rate" form:"rate"`

	Image     string         `json:"image" json:"-"` // Không dùng form:"image"
	SubImages []ProductImage `json:"sub_images"`

	// Trường ẩn/Xử lý logic
	OfferStart         time.Time              `json:"-"`
	OfferEnd           time.Time              `json:"-"`
	ModelID            sql.NullInt64          `json:"-"`
	FileName           sql.NullString         `json:"-"`
	Categories         []Category             `json:"categories"`
	RelatedProducts    []uint64               `json:"related_products" form:"related_products[]"`
	CategoryIDs        []uint32               `json:"category_ids" form:"category_ids[]"`
	DeleteSubImageIDs  []int64                `json:"delete_sub_image_ids" form:"delete_sub_image_ids[]"`
	Attributes         []GroupedAttribute     `json:"attributes"`
	DeleteAttributeIDs []uint32               `json:"delete_attribute_ids" form:"delete_attribute_ids[]"`
	Variants           []ProductVariantDetail `json:"variants"`
	Discounts          []ProductDiscount
}

type ProductDiscount struct {
	ID        int64   `json:"id"`
	ProductID int64   `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Priority  int     `json:"priority"`
	Price     float64 `json:"price"`
	DateStart *string `json:"date_start"` // Dạng "YYYY-MM-DD" hoặc null
	DateEnd   *string `json:"date_end"`   // Dạng "YYYY-MM-DD" hoặc null
}

type ProductSaveRequest struct {
	ID               uint64            `json:"id" form:"id"`
	Title            map[string]string `json:"title"`             // Bỏ tag form
	Description      map[string]string `json:"description"`       // Bỏ tag form
	ShortDescription map[string]string `json:"short_description"` // Bỏ tag form
	MetaTitle        map[string]string `json:"meta_title"`        // Bỏ tag form
	MetaDescription  map[string]string `json:"meta_description"`  // Bỏ tag form

	BrandID        int64   `json:"brand_id" form:"brand_id"`
	ModelNumber    string  `json:"model_number" form:"model_number"`
	SKU            string  `json:"sku" form:"sku"`
	Condition      string  `json:"condition" form:"condition"`
	StockQuantity  int     `json:"stock_quantity" form:"stock_quantity"`
	PurchasePrice  float64 `json:"purchase_price" form:"purchase_price"`
	SalePrice      float64 `json:"price" form:"price"`
	ShippingWeight float64 `json:"shipping_weight" form:"shipping_weight"`
	Active         bool    `json:"active" form:"active"`
	Slug           string  `json:"slug" form:"slug"`

	LinkVideo          string    `json:"link_video" form:"link_video"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	OfferPrice         float64   `json:"offer_price" form:"offer_price"`
	FinalPrice         float64   `json:"final_price" form:"final_price"`
	HasOffer           bool      `json:"has_offer" form:"has_offer"`
	DiscountPercentage float64   `json:"discount_percentage" form:"discount_percentage"`
	OfferStartStr      string    `json:"offer_start" form:"offer_start"`
	OfferEndStr        string    `json:"offer_end" form:"offer_end"`
	TryOnStatus        int64     `json:"try_on_status" form:"try_on_status"`
	Rate               float64   `json:"rate" form:"rate"`

	Image     string         `json:"image"`
	SubImages []ProductImage `json:"sub_images"`

	// Trường ẩn/Xử lý logic
	OfferStart        time.Time         `json:"-"`
	OfferEnd          time.Time         `json:"-"`
	ModelID           sql.NullInt64     `json:"-"`
	FileName          sql.NullString    `json:"-"`
	Categories        []Category        `json:"categories"`
	RelatedProducts   []uint64          `json:"related_products"`     // Bỏ tag form
	CategoryIDs       []uint32          `json:"category_ids"`         // Bỏ tag form
	DeleteSubImageIDs []int64           `json:"delete_sub_image_ids"` // Bỏ tag form
	Attributes        map[uint32][]int  `json:"attributes"`           // Bỏ tag form
	Variants          []VariantInput    `json:"variants"`             // Bỏ tag form
	Discounts         []ProductDiscount `json:"discounts"`
}

type ProductImage struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

type AttributeInput struct {
	AttributeID   uint32 `json:"attribute_id" form:"attribute_id" binding:"required"`
	AttributeName string `json:"attribute_name" form:"attribute_name"`
	Values        []int  `json:"values" form:"values[]"`
}

type AttributeValueItem struct {
	ProductAttributeID uint64  `json:"product_attribute_id"`
	AttributeValueID   int     `json:"attribute_value_id"`
	ValueName          *string `json:"value_name"`
}

type GroupedAttribute struct {
	AttributeID   uint32               `json:"attribute_id"`
	AttributeName string               `json:"attribute_name"`
	Items         []AttributeValueItem `json:"items"` // Phải là slice để chứa nhiều value_id
}

type VariantInput struct {
	ID             uint64   `json:"id"` // Có khi Edit (0 nếu tạo mới)
	SKU            string   `json:"sku"`
	Price          float64  `json:"price"`
	CompareAtPrice *float64 `json:"compare_at_price"`
	Quantity       int      `json:"quantity"`
	Image          string   `json:"image"`
	IsDefault      bool     `json:"is_default"`
	OptionValueIDs []uint64 `json:"option_value_ids"` // Ví dụ: [10, 21] (Đỏ, XL)
}

type ProductVariantDetail struct {
	ID             uint64   `json:"id"`
	SKU            string   `json:"sku"`
	Price          float64  `json:"price"`
	CompareAtPrice *float64 `json:"compare_at_price"`
	Quantity       int      `json:"quantity"`
	Image          string   `json:"image"`
	IsDefault      bool     `json:"is_default"`
	OptionValueIDs []uint64 `json:"option_value_ids"`
}
