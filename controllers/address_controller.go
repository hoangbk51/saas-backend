package controllers

import (
	"fmt"
	"go-saas/models"
	"go-saas/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func ListCustomerAddresses(c *gin.Context) {
	customerID := c.MustGet("customerID").(int)

	// Hardcode type theo Laravel model
	list, err := services.GetAddresses(c, customerID, "App\\Models\\Tenant\\Customer")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": list})
}

func ListAdminCustomerAddresses(c *gin.Context) {

	// Chuyển từ string sang int64 (base 10, bitSize 64)
	customerID, _ := strconv.Atoi(c.Query("customer_id"))
	list, err := services.GetAddresses(c, customerID, "App\\Models\\Tenant\\Customer")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"data": list})
}

func StoreAddress(c *gin.Context) {
	var payload models.AddressPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		//c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		fmt.Printf("Binding Error: %v\n", err)

		c.JSON(400, gin.H{
			"error":   "Dữ liệu không hợp lệ",
			"details": err.Error(), // Trả về chi tiết để debug cho nhanh
		})
		return
	}
	customerID := c.MustGet("customerID").(int)

	// 3. Tự động set các trường hệ thống
	payload.AddressableID = uint64(customerID)
	payload.AddressableType = `App\Models\Tenant\Customer`

	if err := services.CreateAddress(c, payload); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Lưu địa chỉ thành công"})
}

func StoreAdminAddress(c *gin.Context) {
	var payload models.AddressPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		//c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ"})
		fmt.Printf("Binding Error: %v\n", err)

		c.JSON(400, gin.H{
			"error":   "Dữ liệu không hợp lệ",
			"details": err.Error(), // Trả về chi tiết để debug cho nhanh
		})
		return
	}

	// 3. Tự động set các trường hệ thống
	payload.AddressableType = `App\Models\Tenant\Customer`

	if err := services.CreateAddress(c, payload); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Lưu địa chỉ thành công"})
}

func UpdateAdminAddress(c *gin.Context) {
	// 1. Lấy ID từ URL (ví dụ: /addresses/12)
	addressID := c.Param("id")

	var payload models.AddressPayload
	// 2. Bind dữ liệu mới từ JSON
	if err := c.ShouldBindJSON(&payload); err != nil {
		fmt.Printf("Update Binding Error: %v\n", err)
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ", "details": err.Error()})
		return
	}

	// 3. Lấy customerID từ context (để bảo mật)

	// 4. Gán lại các trường hệ thống để đảm bảo user không "hack" sửa địa chỉ của người khác
	payload.AddressableType = `App\Models\Tenant\Customer`

	// 5. Gọi service để thực thi việc update vào DB
	if err := services.UpdateAddress(c, addressID, payload); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Cập nhật địa chỉ thành công"})
}

func UpdateAddress(c *gin.Context) {
	// 1. Lấy ID từ URL (ví dụ: /addresses/12)
	addressID := c.Param("id")

	var payload models.AddressPayload
	// 2. Bind dữ liệu mới từ JSON
	if err := c.ShouldBindJSON(&payload); err != nil {
		fmt.Printf("Update Binding Error: %v\n", err)
		c.JSON(400, gin.H{"error": "Dữ liệu không hợp lệ", "details": err.Error()})
		return
	}

	// 3. Lấy customerID từ context (để bảo mật)
	val, _ := c.Get("customerID")
	customerID := val.(int)

	// 4. Gán lại các trường hệ thống để đảm bảo user không "hack" sửa địa chỉ của người khác
	payload.AddressableID = uint64(customerID)
	payload.AddressableType = `App\Models\Tenant\Customer`

	// 5. Gọi service để thực thi việc update vào DB
	if err := services.UpdateAddress(c, addressID, payload); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Cập nhật địa chỉ thành công"})
}

func DeleteAddress(c *gin.Context) {
	// 1. Lấy ID địa chỉ từ URL (/api/addresses/:id)
	addressID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	// 2. Lấy thông tin chủ sở hữu (Gỉa sử bạn đã lưu vào Context qua Middleware Auth)
	// Ví dụ: ownerID = 1, ownerType = "App\Models\Customer"
	ownerID := c.GetInt("user_id")
	ownerType := "App\\Models\\Tenant\\Customer"

	// 3. Gọi Service xử lý xóa
	if err := services.DeleteAddress(c, addressID, ownerID, ownerType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "Xóa địa chỉ thành công",
	})
}
