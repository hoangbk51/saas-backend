package services

import (
	"encoding/json"
	"fmt"
	"go-saas/utils"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type TwilioConfig struct {
	AccountSID string
	AuthToken  string
	FromNumber string
	//VerifyServiceSID string
	IsLive  bool
	Channel string // "sms" hoặc "whatsapp"
}

// GetTwilioConfig: Tự động lấy cấu hình Live hoặc Sandbox dựa trên database
func GetTwilioConfig(c *gin.Context) (*TwilioConfig, error) {
	isLive := utils.GetSetting(c, "twilio_is_live", "0") == "1"

	prefix := "twilio_test_"
	if isLive {
		prefix = "twilio_live_"
	}

	accountSid := utils.GetSetting(c, prefix+"account_sid", "")
	authToken := utils.GetSetting(c, prefix+"auth_token", "")
	fromNumber := utils.GetSetting(c, prefix+"from_number", "")
	//verifyServiceSid := utils.GetSetting(c, prefix+"verify_service_sid", "")
	channel := utils.GetSetting(c, prefix+"channel", "sms")
	accountSid = "ACa0efb806060e0a02559f027e3b957a65"
	authToken = "5b5cccdde079906445d52b87a5f799ee" // env('TWILIO_AUTH_TOKEN'); 5b5cccdde079906445d52b87a5f799ee
	fromNumber = "+16282903102"                    // "whatsapp:+14155238886"           // "+14155238886"                    //env('TWILIO_WHATSAPP_FROM');//whatsapp:+
	utils.LogSQL("GetTwilioConfig")
	//16282903102
	if accountSid == "" || authToken == "" {
		return nil, fmt.Errorf("chưa cấu hình Twilio API Key (%s)", prefix)
	}

	return &TwilioConfig{
		AccountSID: accountSid,
		AuthToken:  authToken,
		FromNumber: fromNumber,
		//VerifyServiceSID: verifyServiceSid,
		IsLive:  isLive,
		Channel: channel,
	}, nil
}

func FormatE164(phone string) string {
	// Xóa khoảng trắng, dấu gạch ngang, ngoặc
	re := regexp.MustCompile(`[\s\-\(\)]`)
	phone = re.ReplaceAllString(phone, "")

	// Xóa prefix whatsapp: nếu đã có sẵn để xử lý sạch
	phone = strings.TrimPrefix(phone, "whatsapp:")

	// Chuyển 0382305159 -> +84382305159
	if strings.HasPrefix(phone, "0") {
		return "+84" + phone[1:]
	}
	// Chuyển 84382305159 -> +84382305159
	if strings.HasPrefix(phone, "84") {
		return "+" + phone
	}
	// Thêm dấu + nếu chưa có
	if !strings.HasPrefix(phone, "+") {
		return "+" + phone
	}
	return phone
}

// SendTwilioSMS: Gửi SMS thông thường
func SendTwilioSMS(c *gin.Context, toPhone string, messageBody string) error {
	cfg, err := GetTwilioConfig(c)
	if err != nil {
		return err
	}

	if cfg.FromNumber == "" {
		return fmt.Errorf("chưa cấu hình số điện thoại gửi (From Number)")
	}

	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", cfg.AccountSID)

	v := url.Values{}

	cleanFrom := FormatE164(cfg.FromNumber)
	cleanTo := FormatE164(toPhone)

	// 2. Kiểm tra xem có dùng WhatsApp không
	// (Dựa vào setting Channel == "whatsapp" HOẶC cfg.FromNumber trong DB có chứa "whatsapp:")
	isWhatsApp := strings.ToLower(cfg.Channel) == "whatsapp" || strings.HasPrefix(strings.ToLower(cfg.FromNumber), "whatsapp:")

	finalFrom := cleanFrom
	finalTo := cleanTo

	if isWhatsApp {
		finalFrom = "whatsapp:" + cleanFrom
		finalTo = "whatsapp:" + cleanTo
	}

	// 3. Set vào url.Values đúng chuẩn
	v.Set("To", finalTo)
	v.Set("From", finalFrom)

	v.Set("Body", messageBody)

	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", endpoint, strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}

	req.SetBasicAuth(cfg.AccountSID, cfg.AuthToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("lỗi kết nối Twilio: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return fmtSprintfErrorTwilio(resp.StatusCode, errResp)
	}

	return nil
}

func fmtSprintfErrorTwilio(statusCode int, errResp map[string]interface{}) error {
	if msg, ok := errResp["message"].(string); ok {
		return fmt.Errorf("twilio error (%d): %s", statusCode, msg)
	}
	return fmt.Errorf("twilio error (%d)", statusCode)
}
