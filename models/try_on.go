package models

import "github.com/gin-gonic/gin"

// TutaowebRequest: Struct payload gửi sang Provider Tutaoweb
type TutaowebRequest struct {
	ModelImage   string `json:"model_image"`
	GarmentImage string `json:"garment_image"`
	Category     string `json:"category"`
	Mode         string `json:"mode"`
}

// TutaowebResponse: Struct response nhận về từ Provider Tutaoweb
type TutaowebResponse struct {
	Status           string  `json:"status"`
	TaskID           string  `json:"task_id"`
	ResultURL        string  `json:"result_url"`
	ResultImage      string  `json:"result_image"`
	Category         string  `json:"category"`
	Mode             string  `json:"mode"`
	QualityScore     float64 `json:"quality_score"`
	ExecutionTimeMS  int64   `json:"execution_time_ms"`
	RemainingCredits int     `json:"remaining_credits"`
	CreatedAt        string  `json:"created_at"`
	Error            string  `json:"error,omitempty"`
}

type VirtualTryOnRequest struct {
	CustomerImage string `json:"customer_image" binding:"required,url"`
	ProductID     int64  `json:"product_id" binding:"required"`
	ProductImage  string `json:"product_image" binding:"required,url"`
	ProductName   string `json:"product_name"`
	Category      string `json:"category"` // upper_body, lower_body, dresses
}

// TryOnResult: Struct chuẩn hóa kết quả từ các Provider khác nhau
type TryOnResult struct {
	ResultURL string      `json:"result_image"`
	TaskID    string      `json:"task_id,omitempty"`
	RawData   interface{} `json:"raw_data"`
}

// TryOnProvider: Interface chung cho tất cả Provider
type TryOnProvider interface {
	Execute(c *gin.Context, req VirtualTryOnRequest) (*TryOnResult, error)
}
