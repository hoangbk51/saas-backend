package models

import (
	"time"
)

type TenantPool struct {
	DB         any
	LastActive time.Time
}

type PaginationLink struct {
	URL    interface{} `json:"url"`
	Label  string      `json:"label"`
	Active bool        `json:"active"`
}

type LaravelCollection struct {
	Data  interface{} `json:"data"`
	Links struct {
		First interface{} `json:"first"`
		Last  interface{} `json:"last"`
		Prev  interface{} `json:"prev"`
		Next  interface{} `json:"next"`
	} `json:"links"`
	Meta struct {
		CurrentPage int              `json:"current_page"`
		From        int              `json:"from"`
		LastPage    int              `json:"last_page"`
		Links       []PaginationLink `json:"links"`
		Path        string           `json:"path"`
		PerPage     int              `json:"per_page"`
		To          int              `json:"to"`
		Total       int              `json:"total"`
	} `json:"meta"`
}

var (
	allowedTables = map[string]bool{
		"categories":          true,
		"blogs":               true,
		"users":               true,
		"pages":               true,
		"customers":           true,
		"settings":            true,
		"products":            true,
		"attributes":          true,
		"options":             true,
		"coupons":             true,
		"orders":              true,
		"languages":           true,
		"currencies":          true,
		"countries":           true,
		"states":              true,
		"districts":           true,
		"tax_classes":         true,
		"tax_rates":           true,
		"zones":               true,
		"blog_categories":     true,
		"filters":             true,
		"inventories":         true,
		"inventory_adjusts":   true,
		"inventory_histories": true,
		"newsletters":         true,
		"payment_methods":     true,
		"plugins":             true,
		"purchases":           true,
		"purchase_returns":    true,
		"reviews":             true,
		"roles":               true,
		"transactions":        true,
		"warehouses":          true,
		"suppliers":           true,
		"transfers":           true,
		"adjustments":         true,
		"expenses":            true,
	}
)

var (
	tableSchemas = map[string][]string{
		"activity_log":          {"log_name", "description", "subject_id", "subject_type", "causer_id", "causer_type", "properties"},
		"addons":                {"name", "status"},
		"address_types":         {"type"},
		"addresses":             {"type", "address_title", "address_line_1", "address_line_2", "city", "state_id", "zip_code", "country_id", "phone", "latitude", "longitude", "addressable_id", "addressable_type", "is_default"},
		"adjustment_items":      {"adjustment_id", "product_id", "quantity", "method_type"},
		"adjustments":           {"date", "reference_code", "warehouse_id", "total_products"},
		"announcements":         {"user_id", "body", "action_text", "action_url"},
		"attachments":           {"path", "name", "extension", "size", "attachable_id", "attachable_type"},
		"attribute_category":    {"category_id", "attribute_id"},
		"attribute_product":     {"attribute_id", "product_id", "attribute_value_id"},
		"attribute_types":       {"type"},
		"attribute_values":      {"name", "color", "attribute_id", "sort_order"},
		"attributes":            {"name", "attribute_type_id", "order", "filterable"},
		"banner_groups":         {"name"},
		"banners":               {"title", "description", "link", "link_label", "bg_color", "group_id", "columns", "order"},
		"block_templates":       {"name", "setting", "image", "category_id"},
		"blocks":                {"name", "block_template_id", "data", "template_name", "theme_id", "setting"},
		"blog_blog_category":    {"blog_category_id", "blog_id"},
		"blog_categories":       {"parent_id", "name", "slug", "description", "active", "featured", "left", "right"},
		"blog_comments":         {"content", "blog_id", "user_id", "parent", "approved", "likes", "dislikes"},
		"blogs":                 {"title", "slug", "excerpt", "content", "user_id", "status", "approved", "published_at", "likes", "dislikes"},
		"brands":                {"title", "description", "link", "order"},
		"builders":              {"data", "code"},
		"carriers":              {"tax_id", "name", "email", "phone", "tracking_url", "active", "setting", "description", "code"},
		"cart_items":            {"cart_id", "product_id", "item_description", "quantity", "unit_price", "option"},
		"carts":                 {"customer_id", "ip_address", "shipping_country_id", "shipping_zone_id", "shipping_rate_id", "packaging_id", "item_count", "quantity", "total", "discount", "shipping", "packaging", "handling", "taxes", "grand_total", "taxrate", "shipping_weight", "billing_address", "shipping_address", "coupon_id", "payment_status", "payment_method_id", "message_to_customer", "admin_note", "shipping_method_code", "shipping_state_id"},
		"categories":            {"parent_id", "name", "slug", "description", "active", "featured", "left", "right"},
		"category_filter":       {"category_id", "filter_id"},
		"category_product":      {"category_id", "product_id"},
		"cities":                {"name", "type"},
		"compares":              {"product_id", "customer_id"},
		"contact_us":            {"name", "phone", "email", "subject", "message", "read"},
		"countries":             {"capital", "citizenship", "country_code", "currency", "currency_code", "currency_sub_unit", "currency_symbol", "full_name", "iso_3166_2", "iso_3166_3", "name", "region_code", "sub_region_code", "eea", "calling_code", "flag", "active"},
		"coupon_customer":       {"coupon_id", "customer_id"},
		"coupons":               {"name", "code", "description", "value", "min_order_amount", "type", "quantity", "quantity_per_customer", "starting_time", "ending_time", "active"},
		"currencies":            {"priority", "iso_code", "name", "symbol", "symbol_first", "decimal_mark", "thousands_separator", "active", "exchange_rate"},
		"customer_histories":    {"customer_id", "comment"},
		"customer_ips":          {"customer_id", "ip"},
		"customer_transactions": {"customer_id", "order_id", "description", "amount"},
		"customers":             {"name", "nice_name", "email", "password", "dob", "sex", "description", "last_visited_at", "last_visited_from", "stripe_id", "card_holder_name", "card_brand", "card_last_four", "active", "accepts_marketing", "verification_token", "remember_token", "phone"},
		"dashboard_configs":     {"upgrade_plan_notice"},
		"dispute_types":         {"detail"},
		"disputes":              {"dispute_type_id", "customer_id", "order_id", "product_id", "description", "order_received", "return_goods", "refund_amount", "status"},
		"products":              {"slug", "title", "description", "sale_price", "category_id", "id", "brand_id", "title", "model_number", "mpn", "gtin", "gtin_type", "description", "origin_country", "has_variant", "requires_shipping", "downloadable", "warehouse_id", "supplier_id", "sku", "condition", "condition_note", "key_features", "stock_quantity", "damaged_quantity", "user_id", "purchase_price", "sale_price", "offer_price", "offer_start", "offer_end", "shipping_weight", "free_shipping", "available_from", "min_order_quantity", "linked_items", "stuff_pick", "slug", "meta_title", "meta_description", "sale_count", "active", "rating", "video", "short_description", "is_new", "is_hot", "link_video", "tax_class_id", "view", "reward_point", "try_on_status", "try_on_provider_id"},
		"purchase_details":      {"product_id", "quantity", "purchase_price", "sku", "purchase_id"},
		"purchase_items":        {"purchase_id", "product_id", "product_cost", "net_unit_cost", "tax_type", "tax_value", "tax_amount", "discount_type", "discount_value", "discount_amount", "purchase_unit", "quantity", "sub_total"},
		"purchase_returns":      {"date", "supplier_id", "warehouse_id", "tax_rate", "tax_amount", "discount", "shipping", "grand_total", "received_amount", "paid_amount", "payment_type", "status", "payment_status", "notes", "reference_code"},
		"purchases":             {"purchases_number", "warehouse_id", "supplier_id", "payment_status", "stock_status", "note", "total", "debt", "total_paid", "status", "date", "tax_rate", "tax_amount", "shipping", "discount"},
		"refunds":               {"order_id", "order_fulfilled", "amount", "description", "status"},
		"replies":               {"reply", "user_id", "customer_id", "read", "repliable_id", "repliable_type"},
		"reviews":               {"customer_id", "rating", "comment", "product_id", "approved", "spam"},
		"reward_points":         {"customer_id", "order_id", "total", "status"},
		"roles":                 {"name", "guard_name", "level", "description"},
		"suppliers":             {"name", "email", "contact_person", "url", "description"},
		"transfers":             {"date", "id", "date", "from_warehouse_id", "to_warehouse_id", "tax_rate", "tax_amount", "discount", "shipping", "grand_total", "status", "note", "reference_code"},
		"expenses":              {"title", "amount", "category", "description", "expense_date"},
	}
)
