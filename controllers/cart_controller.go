package controllers

import (
	"database/sql"
	"go-saas/models"
	"go-saas/services"
	"go-saas/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func LoadCart(c *gin.Context) {
	utils.LogToFile("LoadCart")
	clientIP := c.ClientIP()

	// Gọi service lấy giỏ hàng kèm theo kết quả Recalculate
	result, err := services.GetCart(c, clientIP)

	if err != nil {
		if err == sql.ErrNoRows {
			// Khách chưa có giỏ hàng
			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"cart":   nil,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Lỗi hệ thống khi lấy giỏ hàng",
			"message": err.Error(),
		})
		return
	}

	// Trả về theo cấu trúc RecalculateResult (Count, Items, Total, Charges, Data)
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"cart":   result,
	})
}

func CartUpdateQuantity(c *gin.Context) {
	// Giả sử URL: /cart/update-item/:id

	// Lấy số lượng mới từ body
	var input struct {
		ID       int64 `json:"id" binding:"required"`
		Quantity int   `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	// Gọi service xử lý
	result, err := services.UpdateItemQuantity(c, input.ID, input.Quantity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Không thể cập nhật giỏ hàng",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Đã cập nhật số lượng",
		"cart":    result, // Trả về RecalculateResult để frontend cập nhật UI ngay lập tức
	})
}

func RemoveCartItem(c *gin.Context) {
	var input struct {
		ID int64 `json:"id" binding:"required"`
	}

	// 1. Bind dữ liệu từ JSON Payload
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID sản phẩm không hợp lệ"})
		return
	}

	// 2. Gọi Service xử lý xóa
	result, err := services.RemoveItemFromCart(c, input.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Không thể xóa sản phẩm",
			"message": err.Error(),
		})
		return
	}

	// 3. Trả về kết quả thành công kèm giỏ hàng mới
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Đã xóa sản phẩm khỏi giỏ hàng",
		"cart":    result,
	})
}

func AddressChange(c *gin.Context) {
	var input models.AddressUpdatePayload

	// BindJSON sẽ quét dựa trên tag ,string chúng ta đã đặt ở Struct
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.LogSQL("Lỗi Bind JSON chi tiết: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":      "error",
			"message":     "Dữ liệu địa chỉ không đúng định dạng",
			"debug_error": err.Error(), // Trả về để bạn check chính xác trường nào lỗi
		})
		return
	}

	// Gọi Service xử lý
	result, methods, err := services.UpdateCartAddress(c, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "success",
		"shippingMethods": methods,
		"cart":            result,
	})
}

func CartUpdate(c *gin.Context) {
	var input models.CartUpdate

	// BindJSON sẽ quét dựa trên tag ,string chúng ta đã đặt ở Struct
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.LogSQL("Lỗi Bind JSON chi tiết: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":      "error",
			"message":     "Dữ liệu cart update không đúng định dạng",
			"debug_error": err.Error(), // Trả về để bạn check chính xác trường nào lỗi
		})
		return
	}

	// Gọi Service xử lý
	result, methods, err := services.CartUpdate(c, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          "success",
		"shippingMethods": methods,
		"cart":            result,
	})
}

/*
func UpdateCartAddressHandler(c *gin.Context) {
	cartID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID giỏ hàng không hợp lệ"})
		return
	}

	var req services.UpdateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu địa chỉ không đúng định dạng"})
		return
	}

	updatedCart, err := services.UpdateCartAddress(c, cartID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Cập nhật địa chỉ và tính lại giỏ hàng thành công",
		"cart":    updatedCart,
	})
}
*/
