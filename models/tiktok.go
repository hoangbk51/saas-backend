package models

// TikTok Order DTOs
type TikTokSearchOrdersResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      struct {
		Orders []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			CreateTime int64  `json:"create_time"`
		} `json:"orders"`
		TotalCount    int    `json:"total_count"`
		NextPageToken string `json:"next_page_token"`
	} `json:"data"`
}

type TikTokOrderDetailResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      struct {
		Orders []TikTokOrder `json:"orders"`
	} `json:"data"`
}

type TikTokOrder struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	BuyerEmail string `json:"buyer_email"`
	Payment    struct {
		TotalAmount float64 `json:"total_amount"`
		Currency    string  `json:"currency"`
	} `json:"payment"`
	ItemList []struct {
		SKUID    string `json:"sku_id"`
		Quantity int    `json:"quantity"`
		SKUName  string `json:"sku_name"`
		Price    string `json:"price"`
	} `json:"item_list"`
	TrackingNumber string `json:"tracking_number"`
	CreateTime     int64  `json:"create_time"`
}

type TikTokSearchProductsResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      struct {
		Products   []TikTokProduct `json:"products"`
		TotalCount int             `json:"total_count"`
	} `json:"data"`
}

// TikTokProduct đại diện sản phẩm từ TikTok
type TikTokProduct struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Status string      `json:"status"`
	SKUs   []TikTokSKU `json:"skus"`
}

// TikTok Webhook DTO
type TikTokWebhookPayload struct {
	Type      int    `json:"type"` // 1: Order Status Update, 2: Stock Update
	ShopID    string `json:"shop_id"`
	Timestamp int64  `json:"timestamp"`
	Data      struct {
		OrderID     string `json:"order_id"`
		OrderStatus string `json:"order_status"`
	} `json:"data"`
}

// TikTokSKU đại diện cho biến thể sản phẩm
type TikTokSKU struct {
	ID    string `json:"id"`
	Price struct {
		OriginalPrice string `json:"original_price"`
	} `json:"price"`
	StockInfos []struct {
		AvailableStock int `json:"available_stock"`
	} `json:"stock_infos"`
}

type ManualMapSKURequest struct {
	StoreID       uint64 `json:"store_id" binding:"required"`
	ChannelType   string `json:"channel_type" binding:"required"` // 'SHOPEE' hoặc 'TIKTOK'
	ChannelSKUID  string `json:"channel_sku_id" binding:"required"`
	ProductID     uint64 `json:"product_id" binding:"required"`
	VariantID     uint64 `json:"variant_id" binding:"required"`
	SyncInventory bool   `json:"sync_inventory"`
	SyncPrice     bool   `json:"sync_price"`
}
