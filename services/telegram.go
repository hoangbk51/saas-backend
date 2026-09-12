package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	botToken = "8995082152:AAE7Env09tcjc3OBDSfk2FJyuBgNcsWuOZo" // Thay bằng Token của Bot
	chatID   = "8980170635"                                     // Thay bằng Chat ID của bạn
)

type TelegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"` // Để dùng định dạng HTML (đậm, nghiêng...)
}

func SendTelegramMessage(message string) error {
	//url := fmt.Sprintf("https://telegram.org", botToken)
	url := "https://api.telegram.org/bot" + botToken + "/sendMessage"
	// Tạo dữ liệu payload
	payload := TelegramPayload{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "HTML",
	}

	// ĐÃ SỬA: Thêm dấu : trước dấu =
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Tạo HTTP Client với thời gian timeout
	client := &http.Client{Timeout: 10 * time.Second}

	// Gửi request POST lên Telegram
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Kiểm tra nếu mã trạng thái không phải 200 OK
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status: %s", resp.Status)
	}

	return nil
}

type TelegramOtpPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// TelegramResponse định nghĩa cấu trúc phản hồi từ Telegram API
type TelegramResponse struct {
	Ok          bool   `json:"ok"`
	Description string `json:"description"`
}

// SendTelegramOTP gửi mã OTP tới Chat ID cụ thể qua Telegram Bot
// Trả về true nếu gửi thành công, false nếu thất bại kèm theo error
func SendTelegramOTP(otpCode string) (bool, error) {
	// Định dạng nội dung tin nhắn bằng Markdownv2 (Cần escape các ký tự đặc biệt nếu có)
	message := fmt.Sprintf("🔒 *Mã OTP xác nhận đăng ký của bạn là:* \n\n🔥 *%s* 🔥\n\nMã có hiệu lực trong 5 phút\\. Vui lòng không chia sẻ mã này\\.", otpCode)

	// Tạo URL gọi API
	url := "https://api.telegram.org/bot" + botToken + "/sendMessage"

	// Khởi tạo payload dữ liệu
	payload := TelegramOtpPayload{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "MarkdownV2", // Giúp hiển thị chữ đậm, nghiêng
	}

	// Chuyển payload sang dạng JSON bytes
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("lỗi mã hóa JSON: %v", err)
	}

	// Khởi tạo HTTP Client với thời gian chờ (timeout) 10 giây
	client := &http.Client{Timeout: 10 * time.Second}

	// Tạo request POST
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Errorf("lỗi tạo request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Thực thi request
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("lỗi kết nối API: %v", err)
	}
	defer resp.Body.Close()

	// Giải mã phản hồi (Response) từ Telegram
	var tgResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return false, fmt.Errorf("lỗi giải mã phản hồi: %v", err)
	}

	// Kiểm tra trạng thái phản hồi từ Telegram
	if resp.StatusCode == http.StatusOK && tgResp.Ok {
		return true, nil
	}

	return false, fmt.Errorf("telegram trả về lỗi: %s (Status: %d)", tgResp.Description, resp.StatusCode)
}
