package models

import "time"

// Warehouse model
type Warehouse struct {
	ID          uint32     `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Email       *string    `json:"email" db:"email"`
	Incharge    *uint64    `json:"incharge" db:"incharge"`
	Description *string    `json:"description" db:"description"`
	Active      bool       `json:"active" db:"active"`
	DeletedAt   *time.Time `json:"deleted_at" db:"deleted_at"`
	CreatedAt   *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at" db:"updated_at"`
	StateID     uint64     `json:"state_id" db:"state_id"`

	// Joined/computed fields
	TotalProducts *int     `json:"total_products,omitempty"`
	TotalStock    *float64 `json:"total_stock,omitempty"`
}

// ManageStock model
type ManageStock struct {
	ID          uint64     `json:"id" db:"id"`
	WarehouseID uint64     `json:"warehouse_id" db:"warehouse_id"`
	ProductID   uint64     `json:"product_id" db:"product_id"`
	VariantID   *uint64    `json:"variant_id" db:"variant_id"`
	Quantity    float64    `json:"quantity" db:"quantity"`
	CreatedAt   *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at" db:"updated_at"`
	Alert       int        `json:"alert" db:"alert"`

	// Joined fields
	ProductName   string `json:"product_name,omitempty" db:"product_name"`
	ProductSKU    string `json:"sku,omitempty" db:"sku"`
	WarehouseName string `json:"warehouse_name,omitempty" db:"warehouse_name"`
}

// Supplier model
type Supplier struct {
	ID            uint32     `json:"id" db:"id"`
	Name          string     `json:"name" db:"name"`
	Email         *string    `json:"email" db:"email"`
	ContactPerson *string    `json:"contact_person" db:"contact_person"`
	URL           *string    `json:"url" db:"url"`
	Description   *string    `json:"description" db:"description"`
	Active        int        `json:"active" db:"active"`
	DeletedAt     *time.Time `json:"deleted_at" db:"deleted_at"`
	CreatedAt     *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at" db:"updated_at"`
}

// Purchase model
type Purchase struct {
	ID              uint64     `json:"id" db:"id"`
	PurchasesNumber *string    `json:"purchases_number" db:"purchases_number"`
	ReferenceCode   *string    `json:"reference_code,omitempty"`
	WarehouseID     *uint32    `json:"warehouse_id" db:"warehouse_id"`
	SupplierID      *uint32    `json:"supplier_id" db:"supplier_id"`
	PaymentStatus   int        `json:"payment_status" db:"payment_status"`
	StockStatus     uint32     `json:"stock_status" db:"stock_status"`
	Note            *string    `json:"note" db:"note"`
	Total           *string    `json:"total" db:"total"`
	Debt            *string    `json:"debt" db:"debt"`
	DeletedAt       *time.Time `json:"deleted_at" db:"deleted_at"`
	CreatedAt       *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at" db:"updated_at"`
	TotalPaid       float64    `json:"total_paid" db:"total_paid"`
	Status          int        `json:"status" db:"status"`
	Date            *string    `json:"date" db:"date"`
	TaxRate         *float64   `json:"tax_rate" db:"tax_rate"`
	TaxAmount       *float64   `json:"tax_amount" db:"tax_amount"`
	Shipping        *float64   `json:"shipping" db:"shipping"`
	Discount        *float64   `json:"discount" db:"discount"`

	// Relational
	WarehouseName string         `json:"warehouse_name,omitempty"`
	SupplierName  string         `json:"supplier_name,omitempty"`
	Items         []PurchaseItem `json:"items,omitempty"`
}

// PurchaseItem model
type PurchaseItem struct {
	ID             uint64     `json:"id" db:"id"`
	PurchaseID     uint64     `json:"purchase_id" db:"purchase_id"`
	ProductID      uint64     `json:"product_id" db:"product_id"`
	ProductCost    *float64   `json:"product_cost" db:"product_cost"`
	NetUnitCost    *float64   `json:"net_unit_cost" db:"net_unit_cost"`
	TaxType        int        `json:"tax_type" db:"tax_type"`
	TaxValue       *float64   `json:"tax_value" db:"tax_value"`
	TaxAmount      *float64   `json:"tax_amount" db:"tax_amount"`
	DiscountType   int        `json:"discount_type" db:"discount_type"`
	DiscountValue  *float64   `json:"discount_value" db:"discount_value"`
	DiscountAmount *float64   `json:"discount_amount" db:"discount_amount"`
	PurchaseUnit   int        `json:"purchase_unit" db:"purchase_unit"`
	Quantity       *float64   `json:"quantity" db:"quantity"`
	SubTotal       *float64   `json:"sub_total" db:"sub_total"`
	CreatedAt      *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at" db:"updated_at"`

	ProductName string `json:"product_name,omitempty"`
}

// PurchaseDetail model
type PurchaseDetail struct {
	ID            uint64     `json:"id" db:"id"`
	ProductID     uint64     `json:"product_id" db:"product_id"`
	Quantity      int        `json:"quantity" db:"quantity"`
	PurchasePrice *float64   `json:"purchase_price" db:"purchase_price"`
	SKU           *string    `json:"sku" db:"sku"`
	PurchaseID    *uint32    `json:"purchase_id" db:"purchase_id"`
	DeletedAt     *time.Time `json:"deleted_at" db:"deleted_at"`
	CreatedAt     *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at" db:"updated_at"`
}

// PurchaseReturn model
type PurchaseReturn struct {
	ID             uint64     `json:"id" db:"id"`
	Date           string     `json:"date" db:"date"`
	SupplierID     uint64     `json:"supplier_id" db:"supplier_id"`
	WarehouseID    uint64     `json:"warehouse_id" db:"warehouse_id"`
	TaxRate        *float64   `json:"tax_rate" db:"tax_rate"`
	TaxAmount      *float64   `json:"tax_amount" db:"tax_amount"`
	Discount       *float64   `json:"discount" db:"discount"`
	Shipping       *float64   `json:"shipping" db:"shipping"`
	GrandTotal     *float64   `json:"grand_total" db:"grand_total"`
	ReceivedAmount *float64   `json:"received_amount" db:"received_amount"`
	PaidAmount     *float64   `json:"paid_amount" db:"paid_amount"`
	PaymentType    *int       `json:"payment_type" db:"payment_type"`
	Status         *int       `json:"status" db:"status"`
	PaymentStatus  *int       `json:"payment_status" db:"payment_status"`
	Notes          *string    `json:"notes" db:"notes"`
	ReferenceCode  *string    `json:"reference_code" db:"reference_code"`
	CreatedAt      *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at" db:"updated_at"`

	WarehouseName string               `json:"warehouse_name,omitempty"`
	SupplierName  string               `json:"supplier_name,omitempty"`
	Items         []PurchaseReturnItem `json:"items,omitempty"`
}

// PurchaseReturnItem model
type PurchaseReturnItem struct {
	ID               uint64     `json:"id" db:"id"`
	PurchaseReturnID uint64     `json:"purchase_return_id" db:"purchase_return_id"`
	ProductID        uint64     `json:"product_id" db:"product_id"`
	ProductCost      *float64   `json:"product_cost" db:"product_cost"`
	NetUnitCost      *float64   `json:"net_unit_cost" db:"net_unit_cost"`
	TaxType          int        `json:"tax_type" db:"tax_type"`
	TaxValue         *float64   `json:"tax_value" db:"tax_value"`
	TaxAmount        *float64   `json:"tax_amount" db:"tax_amount"`
	DiscountType     int        `json:"discount_type" db:"discount_type"`
	DiscountValue    *float64   `json:"discount_value" db:"discount_value"`
	DiscountAmount   *float64   `json:"discount_amount" db:"discount_amount"`
	PurchaseUnit     int        `json:"purchase_unit" db:"purchase_unit"`
	Quantity         *float64   `json:"quantity" db:"quantity"`
	SubTotal         *float64   `json:"sub_total" db:"sub_total"`
	CreatedAt        *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        *time.Time `json:"updated_at" db:"updated_at"`

	ProductName string `json:"product_name,omitempty"`
}

// Transfer model
type Transfer struct {
	ID              uint64     `json:"id" db:"id"`
	Date            string     `json:"date" db:"date"`
	FromWarehouseID uint64     `json:"from_warehouse_id" db:"from_warehouse_id"`
	ToWarehouseID   uint64     `json:"to_warehouse_id" db:"to_warehouse_id"`
	TaxRate         *float64   `json:"tax_rate" db:"tax_rate"`
	TaxAmount       *float64   `json:"tax_amount" db:"tax_amount"`
	Discount        *float64   `json:"discount" db:"discount"`
	Shipping        *float64   `json:"shipping" db:"shipping"`
	GrandTotal      *float64   `json:"grand_total" db:"grand_total"`
	Status          *int       `json:"status" db:"status"`
	Note            *string    `json:"note" db:"note"`
	ReferenceCode   *string    `json:"reference_code" db:"reference_code"`
	CreatedAt       *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at" db:"updated_at"`

	FromWarehouseName string         `json:"from_warehouse_name,omitempty"`
	ToWarehouseName   string         `json:"to_warehouse_name,omitempty"`
	Items             []TransferItem `json:"items,omitempty"`
}

// TransferItem model
type TransferItem struct {
	ID             uint64     `json:"id" db:"id"`
	TransferID     uint64     `json:"transfer_id" db:"transfer_id"`
	ProductID      uint64     `json:"product_id" db:"product_id"`
	ProductCost    *float64   `json:"product_cost" db:"product_cost"`
	NetUnitPrice   *float64   `json:"net_unit_price" db:"net_unit_price"`
	TaxType        int        `json:"tax_type" db:"tax_type"`
	TaxValue       *float64   `json:"tax_value" db:"tax_value"`
	TaxAmount      *float64   `json:"tax_amount" db:"tax_amount"`
	DiscountType   int        `json:"discount_type" db:"discount_type"`
	DiscountValue  *float64   `json:"discount_value" db:"discount_value"`
	DiscountAmount *float64   `json:"discount_amount" db:"discount_amount"`
	Quantity       *float64   `json:"quantity" db:"quantity"`
	SubTotal       *float64   `json:"sub_total" db:"sub_total"`
	CreatedAt      *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at" db:"updated_at"`

	ProductName string `json:"product_name,omitempty"`
}

// Adjustment model
type Adjustment struct {
	ID            uint64     `json:"id" db:"id"`
	Date          string     `json:"date" db:"date"`
	ReferenceCode *string    `json:"reference_code" db:"reference_code"`
	WarehouseID   uint64     `json:"warehouse_id" db:"warehouse_id"`
	TotalProducts *int       `json:"total_products" db:"total_products"`
	CreatedAt     *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at" db:"updated_at"`

	WarehouseName string           `json:"warehouse_name,omitempty"`
	Items         []AdjustmentItem `json:"items,omitempty"`
}

// AdjustmentItem model (method_type: 1 = Addition, 2 = Subtraction)
type AdjustmentItem struct {
	ID           uint64     `json:"id" db:"id"`
	AdjustmentID uint64     `json:"adjustment_id" db:"adjustment_id"`
	ProductID    uint64     `json:"product_id" db:"product_id"`
	Quantity     *float64   `json:"quantity" db:"quantity"`
	MethodType   int        `json:"method_type" db:"method_type"`
	CreatedAt    *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at" db:"updated_at"`

	ProductName string `json:"product_name,omitempty"`
}
