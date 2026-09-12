package utils

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Biến toàn cục dùng chung cho toàn bộ dự án
var RedisClient *redis.Client

func InitRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ Kết nối Redis thất bại: %v", err)
	}

	log.Println("✅ Khởi tạo Redis thành công!")
}
