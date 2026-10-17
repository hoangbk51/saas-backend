package services

type VirtualTryOnRequest struct {
	ProductImage  string      `json:"product_image" form:"product_image"`
	CustomerImage string      `json:"customer_image" form:"customer_image"`
	ProductID     interface{} `json:"product_id" form:"product_id"`
	ProductName   string      `json:"product_name" form:"product_name"`
}
