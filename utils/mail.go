package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/smtp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/gin-gonic/gin"
)

type MailService struct{}

func NewMailService() *MailService {
	return &MailService{}
}

func (s *MailService) SendDynamicEmail(c *gin.Context, toEmail string, templateName string, data map[string]any) error {
	db, err := GetDBFromContext(c)
	if err != nil || db == nil {
		return fmt.Errorf("không lấy được DB từ context: %w", err)
	}

	// 1. Lấy Template từ DB
	var rawSubject, rawBody string
	query := `SELECT subject, body FROM email_templates WHERE name = ? AND deleted_at IS NULL LIMIT 1`
	if err := db.QueryRow(query, templateName).Scan(&rawSubject, &rawBody); err != nil {
		return fmt.Errorf("không tìm thấy email template '%s': %w", templateName, err)
	}

	// 2. Render Body
	tmpl, err := template.New("email").Parse(rawBody)
	if err != nil {
		return fmt.Errorf("lỗi parse body template: %w", err)
	}
	var bodyBuf bytes.Buffer
	if err := tmpl.Execute(&bodyBuf, data); err != nil {
		return fmt.Errorf("lỗi render body: %w", err)
	}

	// 3. Render Subject
	subTmpl, err := template.New("subject").Parse(rawSubject)
	if err != nil {
		return fmt.Errorf("lỗi parse subject template: %w", err)
	}
	var subBuf bytes.Buffer
	if err := subTmpl.Execute(&subBuf, data); err != nil {
		return fmt.Errorf("lỗi render subject: %w", err)
	}

	subject := subBuf.String()
	htmlBody := bodyBuf.String()

	// 4. Lấy Provider từ Setting của Tenant (Mặc định: "smtp")
	provider := GetSetting(c, "default_mail_provider", "smtp")

	// 5. Điều hướng sang đúng Provider handler (Mỗi handler tự load config riêng)
	switch provider {
	case "resend":
		return s.sendViaResend(c, toEmail, subject, htmlBody)
	case "sendgrid":
		return s.sendViaSendGrid(c, toEmail, subject, htmlBody)
	case "aws_ses":
		return s.sendViaAmazonSES(c, toEmail, subject, htmlBody)
	case "smtp":
		fallthrough
	default:
		return s.sendViaSMTP(c, toEmail, subject, htmlBody)
	}
}

// ============================================================================
// PROVIDER HANDLERS (Mỗi provider tự rút config tương ứng từ Tenant DB)
// ============================================================================

// 1. SMTP Provider (Chỉ đọc config SMTP khi provider == "smtp")
func (s *MailService) sendViaSMTP(c *gin.Context, toEmail, subject, htmlBody string) error {
	sender, _ := GetPluginSetting(c, "smtp_sender", "")
	password, _ := GetPluginSetting(c, "smtp_password", "")
	host, _ := GetPluginSetting(c, "smtp_host", "")
	port, _ := GetPluginSetting(c, "smtp_port", "587")

	if sender == "" || host == "" {
		return fmt.Errorf("cấu hình SMTP của tenant chưa đầy đủ (sender/host bị trống)")
	}

	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg := []byte(fmt.Sprintf("Subject: %s\n%s%s", subject, mime, htmlBody))

	auth := smtp.PlainAuth("", sender, password, host)
	addr := fmt.Sprintf("%s:%s", host, port)

	return smtp.SendMail(addr, auth, sender, []string{toEmail}, msg)
}

// 2. Resend Provider
func (s *MailService) sendViaResend(c *gin.Context, toEmail, subject, htmlBody string) error {
	apiKey, _ := GetPluginSetting(c, "resend", "api_key")
	fromEmail, _ := GetPluginSetting(c, "resend", "from_email")

	if apiKey == "" || fromEmail == "" {
		return fmt.Errorf("plugin Resend chưa được cấu hình api_key hoặc from_email")
	}

	payload, _ := json.Marshal(map[string]any{
		"from":    fromEmail,
		"to":      []string{toEmail},
		"subject": subject,
		"html":    htmlBody,
	})

	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", "https://api.resend.com/emails", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend error [%d]: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// 3. SendGrid Provider
func (s *MailService) sendViaSendGrid(c *gin.Context, toEmail, subject, htmlBody string) error {
	apiKey, _ := GetPluginSetting(c, "sendgrid", "api_key")
	fromEmail, _ := GetPluginSetting(c, "sendgrid", "from_email")

	if apiKey == "" || fromEmail == "" {
		return fmt.Errorf("plugin SendGrid chưa được cấu hình api_key hoặc from_email")
	}

	payload, _ := json.Marshal(map[string]any{
		"personalizations": []map[string]any{{"to": []map[string]string{{"email": toEmail}}}},
		"from":             map[string]string{"email": fromEmail},
		"subject":          subject,
		"content":          []map[string]string{{"type": "text/html", "value": htmlBody}},
	})

	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendgrid error [%d]: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// 4. Amazon SES Provider
func (s *MailService) sendViaAmazonSES(c *gin.Context, toEmail, subject, htmlBody string) error {
	accessKey, _ := GetPluginSetting(c, "aws_ses", "access_key_id")
	secretKey, _ := GetPluginSetting(c, "aws_ses", "secret_access_key")
	region, _ := GetPluginSetting(c, "aws_ses", "region")
	fromEmail, _ := GetPluginSetting(c, "aws_ses", "from_email")

	if accessKey == "" || secretKey == "" || fromEmail == "" {
		return fmt.Errorf("plugin AWS SES chưa cấu hình đủ credentials hoặc from_email")
	}
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(c.Request.Context(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return fmt.Errorf("AWS config error: %w", err)
	}

	sesClient := ses.NewFromConfig(cfg)
	_, err = sesClient.SendEmail(c.Request.Context(), &ses.SendEmailInput{
		Source:      aws.String(fromEmail),
		Destination: &sestypes.Destination{ToAddresses: []string{toEmail}},
		Message: &sestypes.Message{
			Subject: &sestypes.Content{Data: aws.String(subject), Charset: aws.String("UTF-8")},
			Body:    &sestypes.Body{Html: &sestypes.Content{Data: aws.String(htmlBody), Charset: aws.String("UTF-8")}},
		},
	})

	return err
}
