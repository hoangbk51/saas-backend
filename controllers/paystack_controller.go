package controllers

import (
	"encoding/json"
	"fmt"
	"go-saas/services"
	"go-saas/utils"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func PaystackCallback(c *gin.Context) {
	reference := c.Query("reference")

	// 1. Lấy Key của Tenant để Verify
	clientSecret := "sk_test_664146b8a1476a175ecd248e1bd42da2ac9adc23"

	// 2. Gọi API Verify của Paystack
	req, _ := http.NewRequest("GET", "https://api.paystack.co/transaction/verify/"+reference, nil)
	req.Header.Set("Authorization", "Bearer "+clientSecret)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	defer resp.Body.Close()

	// 3. Parse kết quả
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	data := result["data"].(map[string]any)
	status := data["status"].(string)
	if status == "success" {
		// 1. Lấy Metadata an toàn
		metadata, ok := data["metadata"].(map[string]any)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Metadata is missing"})
			return
		}

		// 2. Lấy custom_fields dưới dạng MẢNG (Slice) - Đây là chỗ gây lỗi Panic của bạn
		customFieldsArray, ok := metadata["custom_fields"].([]any)
		if !ok {
			// Nếu không phải mảng, có thể tenant này gửi format cũ, kiểm tra lại map
			utils.LogSQL("custom_fields is not an array")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid custom_fields format"})
			return
		}

		var refCode string

		// 3. Duyệt mảng để tìm đúng variable_name là "reference_code"
		for _, field := range customFieldsArray {
			f, ok := field.(map[string]any)
			if !ok {
				continue
			}

			if f["variable_name"] == "reference_code" {
				// Ép kiểu value về string (dùng fmt.Sprintf cho chắc chắn nếu value là số)
				refCode = fmt.Sprintf("%v", f["value"])
				break
			}
		}

		// 4. Log và xử lý logic tiếp theo
		utils.LogSQL("Found refCode: " + refCode)

		if refCode != "" {
			// Cập nhật Database
			err = services.CheckoutCallBack(c, refCode)
			if err != nil {
				c.JSON(500, gin.H{"error": "Update transaction failed"})
				return
			}
		}

		log.Printf("Order %v thanh toán thành công!", refCode)
		c.JSON(http.StatusOK, gin.H{"message": "Thanh toán hoàn tất"})

	} else {
		c.String(http.StatusOK, "fail")
	}
}
