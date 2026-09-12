package services

type ShippingOption struct {
	Code  string  `json:"code"` // ghn, ghtk
	Name  string  `json:"name"` // Giao Hàng Nhanh
	Fee   float64 `json:"fee"`  // 30000
	Error string  `json:"error,omitempty"`
}
