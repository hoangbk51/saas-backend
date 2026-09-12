package models

// ShopeeWebhookPayload cấu trúc nhận sự kiện từ Shopee Webhook
type ShopeeWebhookPayload struct {
	ShopID    int64 `json:"shop_id"`
	Code      int   `json:"code"`
	Timestamp int64 `json:"timestamp"`
	Data      struct {
		OrderSN     string `json:"ordersn"`
		OrderStatus string `json:"order_status"`
		UpdateTime  int64  `json:"update_time"`
	} `json:"data"`
}
