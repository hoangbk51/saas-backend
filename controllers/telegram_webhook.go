package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-saas/utils"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// TelegramUser struct
type TelegramUser struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
}

// TelegramChat struct
type TelegramChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

// TelegramMessage struct
type TelegramMessage struct {
	MessageID int          `json:"message_id"`
	From      TelegramUser `json:"from"`
	Chat      TelegramChat `json:"chat"`
	Date      int64        `json:"date"`
	Text      string       `json:"text"`
	Caption   string       `json:"caption"`
}

// Structural Payload chuẩn của Telegram Webhook Update
type TelegramUpdate struct {
	UpdateID      int              `json:"update_id"`
	Message       *TelegramMessage `json:"message"`
	EditedMessage *TelegramMessage `json:"edited_message"`
}

// Cấu trúc Message đã Chuẩn hóa (Normalized)
type GenericMessage struct {
	Channel        string `json:"channel"`
	TenantID       string `json:"tenant_id"`
	ConversationID int64  `json:"conversation_id,omitempty"`
	MessageID      int64  `json:"message_id,omitempty"`
	SenderID       string `json:"sender_id"`
	SenderName     string `json:"sender_name"`
	Content        string `json:"content"`
	Status         string `json:"status,omitempty"`
}

// Struct để gửi sang Socket.io Server
type SocketEmitPayload struct {
	Event          string         `json:"event"`
	Room           string         `json:"room,omitempty"`
	ConversationID int64          `json:"conversation_id,omitempty"`
	Data           GenericMessage `json:"data"`
}

// Default Telegram Bot Token (có thể ghi đè bằng env TELEGRAM_BOT_TOKEN)
const DefaultTelegramBotToken = "8995082152:AAE7Env09tcjc3OBDSfk2FJyuBgNcsWuOZo"

func getTelegramBotToken() string {
	tok := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tok != "" {
		return tok
	}
	return DefaultTelegramBotToken
}

// EnsureTelegramChatTables đảm bảo cấu trúc bảng hỗ trợ đa kênh (Omnichannel) cho Telegram
func EnsureTelegramChatTables(db *sqlx.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			customer_id bigint unsigned NOT NULL DEFAULT 0,
			assigned_admin_id bigint unsigned DEFAULT NULL,
			status varchar(30) NOT NULL DEFAULT 'ai',
			last_message_at datetime DEFAULT NULL,
			created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			guest_token_hash varchar(64) DEFAULT NULL,
			channel varchar(30) NOT NULL DEFAULT 'web',
			external_chat_id varchar(100) DEFAULT NULL,
			customer_name varchar(255) DEFAULT NULL,
			PRIMARY KEY (id),
			KEY idx_external_chat_id (external_chat_id),
			KEY idx_guest_token_hash (guest_token_hash),
			KEY idx_channel (channel)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
	`)

	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			conversation_id bigint unsigned NOT NULL,
			sender_type varchar(20) NOT NULL,
			sender_id bigint unsigned NOT NULL DEFAULT '0',
			message_type varchar(20) NOT NULL DEFAULT 'text',
			content text NOT NULL,
			channel varchar(30) NOT NULL DEFAULT 'web',
			sender_name varchar(255) DEFAULT NULL,
			is_internal tinyint(1) NOT NULL DEFAULT 0,
			created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			KEY idx_conversation_id (conversation_id),
			KEY idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
	`)

	// Bổ sung các cột nếu thiếu
	alterColumns := []string{
		"ALTER TABLE conversations ADD COLUMN channel varchar(30) NOT NULL DEFAULT 'web'",
		"ALTER TABLE conversations ADD COLUMN external_chat_id varchar(100) DEFAULT NULL",
		"ALTER TABLE conversations ADD COLUMN customer_name varchar(255) DEFAULT NULL",
		"ALTER TABLE messages ADD COLUMN channel varchar(30) NOT NULL DEFAULT 'web'",
		"ALTER TABLE messages ADD COLUMN sender_name varchar(255) DEFAULT NULL",
		"ALTER TABLE messages ADD COLUMN is_internal tinyint(1) NOT NULL DEFAULT 0",
	}
	for _, sqlStmt := range alterColumns {
		_, _ = db.Exec(sqlStmt)
	}

	return nil
}

// 1. POST /api/v2/webhooks/telegram
// Nhận Webhook từ Telegram Server khi khách hàng nhắn tin cho Telegram Bot
func HandleTelegramWebhook(c *gin.Context) {
	utils.LogToFile("� [Telegram Webhook] Received incoming update")

	var update TelegramUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		log.Println("❌ [Telegram Webhook] ShouldBindJSON error:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}

	// Lấy message hoặc edited_message
	msgObj := update.Message
	if msgObj == nil && update.EditedMessage != nil {
		msgObj = update.EditedMessage
	}

	if msgObj == nil {
		// Event không chứa tin nhắn (ví dụ status update, polling ping)
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Ignored non-message update"})
		return
	}

	// Bỏ qua tin nhắn từ Bot để tránh loop
	if msgObj.From.IsBot {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Ignored bot message"})
		return
	}

	// 1. Trích xuất nội dung tin nhắn
	content := strings.TrimSpace(msgObj.Text)
	if content == "" {
		content = strings.TrimSpace(msgObj.Caption)
	}
	if content == "" {
		content = "[Khách hàng đã gửi tệp/hình ảnh hoặc sticker]"
	}

	chatID := fmt.Sprintf("%d", msgObj.Chat.ID)
	senderID := fmt.Sprintf("%d", msgObj.From.ID)

	senderName := strings.TrimSpace(fmt.Sprintf("%s %s", msgObj.From.FirstName, msgObj.From.LastName))
	if senderName == "" {
		senderName = msgObj.From.Username
	}
	if senderName == "" {
		senderName = fmt.Sprintf("Telegram User %s", senderID)
	}

	// 2. Xác định Tenant ID
	tenantID := c.GetString("tenantId")
	if tenantID == "" {
		tenantID = c.GetHeader("x-tenant-id")
	}
	if tenantID == "" {
		tenantID = c.Query("tenant_id")
	}
	if tenantID == "" {
		tenantID = "popshop"
	}

	log.Printf("� [Telegram Webhook] From: %s (ChatID: %s, Tenant: %s): %s\n", senderName, chatID, tenantID, content)

	// 3. Kết nối Database Tenant và Lưu dữ liệu (Conversations & Messages)
	var convID int64
	var convStatus string = "ai"
	guestTokenHash := "tg_" + chatID

	var tenantDB *sqlx.DB
	dbFromCtx, err := utils.GetDBFromContext(c)
	if err == nil && dbFromCtx != nil {
		tenantDB = dbFromCtx
	} else {
		rawDB, dbErr := utils.GetTenantDBByTenantID(tenantID)
		if dbErr == nil && rawDB != nil {
			tenantDB = sqlx.NewDb(rawDB, "mysql")
		}
	}

	if tenantDB != nil {
		_ = EnsureTelegramChatTables(tenantDB)

		// Kiểm tra cuộc hội thoại đã tồn tại cho chat Telegram này chưa
		var existingConv struct {
			ID              int64   `db:"id"`
			Status          string  `db:"status"`
			AssignedAdminID *uint64 `db:"assigned_admin_id"`
		}

		err = tenantDB.Get(&existingConv, `
			SELECT id, status, assigned_admin_id 
			FROM conversations 
			WHERE channel = 'telegram' AND (external_chat_id = ? OR guest_token_hash = ?)
			ORDER BY id DESC LIMIT 1
		`, chatID, guestTokenHash)

		if err == nil && existingConv.ID > 0 {
			convID = existingConv.ID
			convStatus = existingConv.Status
			// Cập nhật thời gian tin nhắn mới nhất
			_, _ = tenantDB.Exec(`
				UPDATE conversations 
				SET last_message_at = NOW(), updated_at = NOW(), customer_name = ? 
				WHERE id = ?
			`, senderName, convID)
		} else {
			// Tạo mới cuộc hội thoại với status ban đầu là 'ai'
			res, insertErr := tenantDB.Exec(`
				INSERT INTO conversations (customer_id, status, last_message_at, created_at, updated_at, guest_token_hash, channel, external_chat_id, customer_name)
				VALUES (0, 'ai', NOW(), NOW(), NOW(), ?, 'telegram', ?, ?)
			`, guestTokenHash, chatID, senderName)
			if insertErr == nil {
				convID, _ = res.LastInsertId()
				convStatus = "ai"
			}
		}

		// Lưu tin nhắn của khách hàng vào bảng messages
		if convID > 0 {
			_, _ = tenantDB.Exec(`
				INSERT INTO messages (conversation_id, sender_type, sender_id, message_type, content, channel, sender_name, created_at)
				VALUES (?, 'customer', 0, 'text', ?, 'telegram', ?, NOW())
			`, convID, content, senderName)
		}
	}

	// 4. Nếu là lệnh /start: Gửi tin nhắn chào mừng lập tức về Telegram
	if content == "/start" {
		welcomeText := fmt.Sprintf("� Xin chào %s! Chào mừng bạn đến với cửa hàng.\n\n� Trợ lý AI và đội ngũ tư vấn viên sẵn sàng giải đáp thắc mắc về sản phẩm, giá bán, khuyến mãi và đơn hàng 24/7.\n\nBạn hãy gửi câu hỏi hoặc sản phẩm cần tư vấn nhé!", senderName)
		_ = sendToTelegram(chatID, welcomeText)

		if tenantDB != nil && convID > 0 {
			_, _ = tenantDB.Exec(`
				INSERT INTO messages (conversation_id, sender_type, sender_id, message_type, content, channel, sender_name, created_at)
				VALUES (?, 'ai', 0, 'text', ?, 'telegram', 'Trợ lý AI', NOW())
			`, convID, welcomeText)
		}
	}

	// 5. Chuẩn hóa dữ liệu và gửi sang Socket.io Server (Port 3000)
	// Để kích hoạt:
	// - Hiển thị Realtime tin nhắn lên Admin Chat Panel
	// - Tự động kích hoạt Trợ lý AI RAG (processCustomerMessageWithRag) nếu status == 'ai'
	genericMsg := GenericMessage{
		Channel:        "telegram",
		TenantID:       tenantID,
		ConversationID: convID,
		SenderID:       chatID,
		SenderName:     senderName,
		Content:        content,
		Status:         convStatus,
	}

	go emitToSocketServer("telegram_emit", fmt.Sprintf("admin_chat_room_%s", tenantID), genericMsg)

	// Luôn trả về 200 OK ngay lập tức cho Telegram để tránh Telegram retry liên tục
	c.JSON(http.StatusOK, gin.H{
		"status":          "ok",
		"conversation_id": convID,
		"channel":         "telegram",
	})
}

// emitToSocketServer gửi HTTP POST sang Node.js Socket.IO Server để broadcast realtime và kích hoạt AI RAG
func emitToSocketServer(event string, room string, data GenericMessage) {
	// Danh sách các URL socket internal để thử kết nối
	socketURLs := []string{
		"http://127.0.0.1:3000/internal/emit",
		"http://localhost:3000/internal/emit",
	}

	if customURL := os.Getenv("SOCKET_INTERNAL_URL"); customURL != "" {
		socketURLs = append([]string{customURL}, socketURLs...)
	}

	payload := SocketEmitPayload{
		Event:          event,
		Room:           room,
		ConversationID: data.ConversationID,
		Data:           data,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Println("❌ [Error] Marshal SocketEmitPayload failed:", err)
		return
	}

	client := &http.Client{Timeout: 6 * time.Second}
	var success bool = false

	for _, socketURL := range socketURLs {
		req, err := http.NewRequest("POST", socketURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-tenant-id", data.TenantID)

		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				success = true
				break
			}
		}
	}

	if !success {
		log.Printf("⚠️ [Socket Emit Warning] Could not connect to Socket server at %v", socketURLs)
	}
}

// IncomingSocketMessage cấu trúc nhận từ Socket Server / Admin Chat
type IncomingSocketMessage struct {
	ConversationID string `json:"conversation_id"`
	TelegramChatID string `json:"telegram_chat_id"`
	SenderType     string `json:"sender_type"` // 'ai' hoặc 'admin'
	SenderName     string `json:"sender_name"`
	Content        string `json:"content"`
	TempID         string `json:"temp_id"`
	TenantID       string `json:"tenant_id"`
}

// TelegramSendMessagePayload Struct gửi sang Telegram API
type TelegramSendMessagePayload struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// 2. POST /api/v2/telegram/internal/messages/process
// Nhận tin nhắn từ Server Socket / AI RAG / Admin gửi sang Backend Go rồi đẩy đến Telegram người dùng
func HandleProcessMessage(c *gin.Context) {
	var msg IncomingSocketMessage

	// 1. Parse JSON body
	if err := c.ShouldBindJSON(&msg); err != nil {
		utils.LogToFile("❌ [Telegram Process] Payload error: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Payload không hợp lệ: " + err.Error(),
		})
		return
	}

	utils.LogToFile(fmt.Sprintf("� [Telegram Process] Received message for Conv #%s (ChatID: %s, Sender: %s): %s",
		msg.ConversationID, msg.TelegramChatID, msg.SenderType, msg.Content))

	tenantID := msg.TenantID
	if tenantID == "" {
		tenantID = c.GetString("tenantId")
	}
	if tenantID == "" {
		tenantID = c.GetHeader("x-tenant-id")
	}
	if tenantID == "" {
		tenantID = "popshop"
	}

	// 2. Trích xuất Telegram Chat ID
	chatID := extractTelegramChatID(msg.ConversationID, msg.TelegramChatID)

	// Nếu chatID vẫn rỗng, thử tra cứu từ Database bằng conversation_id
	if chatID == "" && msg.ConversationID != "" {
		var tenantDB *sqlx.DB
		dbFromCtx, err := utils.GetDBFromContext(c)
		if err == nil && dbFromCtx != nil {
			tenantDB = dbFromCtx
		} else {
			rawDB, dbErr := utils.GetTenantDBByTenantID(tenantID)
			if dbErr == nil && rawDB != nil {
				tenantDB = sqlx.NewDb(rawDB, "mysql")
			}
		}

		if tenantDB != nil {
			var extChatID string
			err := tenantDB.Get(&extChatID, "SELECT COALESCE(external_chat_id, guest_token_hash) FROM conversations WHERE id = ? LIMIT 1", msg.ConversationID)
			if err == nil && extChatID != "" {
				chatID = strings.TrimPrefix(extChatID, "tg_")
			}
		}
	}

	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Không thể xác định Telegram Chat ID để gửi tin nhắn",
		})
		return
	}

	// 3. Đồng bộ tin nhắn vào DB Tenant nếu chưa được ghi (đảm bảo tính toàn vẹn)
	var tenantDB *sqlx.DB
	dbFromCtx, err := utils.GetDBFromContext(c)
	if err == nil && dbFromCtx != nil {
		tenantDB = dbFromCtx
	} else {
		rawDB, dbErr := utils.GetTenantDBByTenantID(tenantID)
		if dbErr == nil && rawDB != nil {
			tenantDB = sqlx.NewDb(rawDB, "mysql")
		}
	}

	if tenantDB != nil && msg.ConversationID != "" {
		senderName := msg.SenderName
		if senderName == "" {
			if msg.SenderType == "ai" {
				senderName = "Trợ lý AI"
			} else {
				senderName = "Nhân viên hỗ trợ"
			}
		}

		// Kiểm tra xem tin nhắn này đã vừa được insert hay chưa (tránh trùng lặp)
		var existsCount int
		_ = tenantDB.Get(&existsCount, `
			SELECT COUNT(*) FROM messages 
			WHERE conversation_id = ? AND sender_type = ? AND content = ? 
			  AND created_at >= NOW() - INTERVAL 10 SECOND
		`, msg.ConversationID, msg.SenderType, msg.Content)

		if existsCount == 0 {
			_, _ = tenantDB.Exec(`
				INSERT INTO messages (conversation_id, sender_type, sender_id, message_type, content, channel, sender_name, created_at)
				VALUES (?, ?, 0, 'text', ?, 'telegram', ?, NOW())
			`, msg.ConversationID, msg.SenderType, msg.Content, senderName)

			_, _ = tenantDB.Exec(`
				UPDATE conversations SET last_message_at = NOW(), updated_at = NOW() WHERE id = ?
			`, msg.ConversationID)
		}
	}

	// 4. Gửi sang Telegram Bot API
	err = sendToTelegram(chatID, msg.Content)
	if err != nil {
		log.Printf("❌ [Telegram Process] Send to Telegram error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Lỗi khi gửi sang Telegram API: " + err.Error(),
		})
		return
	}

	log.Printf("✅ [Telegram Process] Message successfully sent to Telegram ChatID %s (Sender: %s)\n", chatID, msg.SenderType)

	// 5. Trả về Response thành công
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message_id":      time.Now().UnixNano(),
		"telegram_chat_id": chatID,
		"sender_type":     msg.SenderType,
	})
}

// Gọi Bot API chính thức của Telegram để chuyển tin nhắn đến người dùng
func sendToTelegram(chatID string, text string) error {
	botToken := getTelegramBotToken()
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := TelegramSendMessagePayload{
		ChatID: chatID,
		Text:   text,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telegram API trả về status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// extractTelegramChatID Hàm bổ trợ lấy Telegram Chat ID thuần (bỏ tiền tố "tg_")
func extractTelegramChatID(convID, explicitChatID string) string {
	if explicitChatID != "" {
		return strings.TrimPrefix(explicitChatID, "tg_")
	}
	if strings.HasPrefix(convID, "tg_") {
		return strings.TrimPrefix(convID, "tg_")
	}
	return convID
}
