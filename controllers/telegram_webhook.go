package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-saas/utils"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Structural Payload chuẩn của Telegram Webhook
type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

// Cấu trúc Message đã Chuẩn hóa (Normalized)
type GenericMessage struct {
	Channel    string `json:"channel"`
	TenantID   string `json:"tenant_id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Content    string `json:"content"`
}

// Struct để gửi sang Socket.io Server
type SocketEmitPayload struct {
	Event string         `json:"event"`
	Room  string         `json:"room"`
	Data  GenericMessage `json:"data"`
}

func HandleTelegramWebhookTest(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})

}
func HandleTelegramWebhook(c *gin.Context) {
	utils.LogToFile("HandleTelegramWebhook")
	var update TelegramUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}
	utils.LogToFile(update)

	if update.Message != nil {
		// 1. Chuẩn hóa dữ liệu từ Telegram
		genericMsg := GenericMessage{
			Channel:    "telegram",
			TenantID:   c.GetString("tenantId"), // Ví dụ Shop ID gán cố định hoặc lookup theo Bot Token
			SenderID:   fmt.Sprintf("%d", update.Message.From.ID),
			SenderName: update.Message.From.FirstName,
			Content:    update.Message.Text,
		}

		log.Printf("[Webhook] Received from %s: %s\n", genericMsg.SenderName, genericMsg.Content)

		// 2. TODO: Lưu vào Database (MySQL/PostgreSQL) ở đây

		// 3. Gọi Internal HTTP POST sang Server Socket.io (Port 3000)
		go emitToSocketServer("telegram_emit", fmt.Sprintf("shop_%s", genericMsg.TenantID), genericMsg)
	}

	// Luôn trả về 200 OK ngay lập tức cho Telegram
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func emitToSocketServer(event string, room string, data GenericMessage) {
	socketURL := "http://localhost:3000/internal/emit"

	payload := SocketEmitPayload{
		Event: event,
		Room:  room,
		Data:  data,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Println("[Error] Marshal payload failed:", err)
		return
	}

	resp, err := http.Post(socketURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Println("[Error] Call Socket server failed:", err)
		return
	}
	defer resp.Body.Close()
}

type IncomingSocketMessage struct {
	ConversationID string `json:"conversation_id"`
	TelegramChatID string `json:"telegram_chat_id"`
	SenderType     string `json:"sender_type"`
	Content        string `json:"content"`
	TempID         string `json:"temp_id"`
}

// Struct gửi sang Telegram API
type TelegramSendMessagePayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

const TelegramBotToken = "8995082152:AAE7Env09tcjc3OBDSfk2FJyuBgNcsWuOZo"

// Handler xử lý tin nhắn
func HandleProcessMessage(c *gin.Context) {
	var msg IncomingSocketMessage

	// 1. Parse JSON body bằng Gin
	if err := c.ShouldBindJSON(&msg); err != nil {
		utils.LogToFile(err)

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Payload không hợp lệ: " + err.Error(),
		})
		return
	}
	utils.LogToFile(msg)
	// 2. Lưu DB (Ví dụ: GORM / SQL)
	// db.Create(&Message{...})

	// 3. Nếu là tin nhắn gửi sang Telegram
	chatID := extractTelegramChatID(msg.ConversationID, msg.TelegramChatID)
	if chatID != "" {
		err := sendToTelegram(chatID, msg.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Lỗi khi gửi sang Telegram API: " + err.Error(),
			})
			return
		}
	}

	// 4. Trả về Response thành công cho Node.js
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message_id": time.Now().UnixNano(),
	})
}

// Gọi Bot API chính thức của Telegram
func sendToTelegram(chatID string, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", TelegramBotToken)

	payload := TelegramSendMessagePayload{
		ChatID: chatID,
		Text:   text,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API trả về status: %d", resp.StatusCode)
	}

	return nil
}

// Hàm bổ trợ lấy Telegram Chat ID thuần (bỏ tiền tố "tg_")
func extractTelegramChatID(convID, explicitChatID string) string {
	if explicitChatID != "" {
		return explicitChatID
	}
	if strings.HasPrefix(convID, "tg_") {
		return strings.TrimPrefix(convID, "tg_")
	}
	return convID
}
