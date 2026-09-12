package controllers

import (
	"fmt"
	"go-saas/services"

	"github.com/gin-gonic/gin"
)

func Test(c *gin.Context) {

	message := "🔔 <b>[Thông báo từ Web]</b>\nCó một đơn hàng mới vừa được tạo thành công!"

	fmt.Println("Đang gửi tin nhắn...")
	err := services.SendTelegramMessage(message)
	if err != nil {
		fmt.Printf("❌ Gửi tin nhắn thất bại: %v\n", err)
		return
	}
	fmt.Println("✅ Gửi tin nhắn thành công!")
}
