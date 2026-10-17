package utils

import (
	"bytes"
	"bytes/json"
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

func (m *MailConfig) SendDynamicEmail(c *gin.Context, toEmail string, templateName string, data map[string]any) error {
	db, err := GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("get db from context: %w", err)
	}

	if db == nil {
		return fmt.Errorf("database is nil")
	}

	// 1. Lấy Template từ DB
	var rawSubject, rawBody string

	query := `
		SELECT subject, body
		FROM email_templates
		WHERE name = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	LogSQL(query, templateName)

	err = db.QueryRow(query, templateName).Scan(
		&rawSubject,
		&rawBody,
	)

	if err != nil {
		return fmt.Errorf("không tìm thấy email template '%s': %w", templateName, err)
	}

	// 2. Render Body bằng html/template
	tmpl, err := template.New("email").Parse(rawBody)
	if err != nil {
		return fmt.Errorf("lỗi parse template body: %v", err)
	}

	var bodyBuffer bytes.Buffer
	if err := tmpl.Execute(&bodyBuffer, data); err != nil {
		return fmt.Errorf("lỗi execute template: %v", err)
	}

	// 3. Render Subject
	subTmpl, err := template.New("subject").Parse(rawSubject)
	if err != nil {
		return fmt.Errorf("lỗi parse template subject: %w", err)
	}
	var subBuffer bytes.Buffer
	if err := subTmpl.Execute(&subBuffer, data); err != nil {
		return fmt.Errorf("lỗi execute subject: %w", err)
	}

	renderedSubject := subBuffer.String()
	renderedBody := bodyBuffer.String()

	// 4. Lấy Provider được cấu hình trong hệ thống (Mặc định: "smtp")
	provider, _ := GetSetting(c, "default_mail_provider", "smtp")

	// 5. Điều hướng xử lý theo Provider
	switch provider {
	case "resend":
		return m.sendViaResend(c, toEmail, renderedSubject, renderedBody)
	case "sendgrid":
		return m.sendViaSendGrid(c, toEmail, renderedSubject, renderedBody)
	case "aws_ses":
		return m.sendViaAmazonSES(c, toEmail, renderedSubject, renderedBody)
	case "smtp":
		fallthrough
	default:
		return m.sendViaSMTP(toEmail, renderedSubject, renderedBody)
	}
}

// ==========================================
// PRIVATE HELPER METHODS FOR MAIL PROVIDERS
// ==========================================

// 1. SendGrid API v3
func (m *MailConfig) sendViaSendGrid(c *gin.Context, toEmail string, subject string, htmlBody string) error {
	apiKey, _ := GetPluginSetting(c, "sendgrid", "api_key")
	if apiKey == "" {
		return fmt.Errorf("sendgrid api_key chưa được cấu hình trong plugin setting")
	}

	fromEmail, _ := GetPluginSetting(c, "sendgrid", "from_email")
	if fromEmail == "" {
		fromEmail = m.Sender
	}

	// Payload theo chuẩn SendGrid Mail Send v3 API
	payload := map[string]any{
		"personalizations": []map[string]any{
			{
				"to": []map[string]string{
					{"email": toEmail},
				},
			},
		},
		"from": map[string]string{
			"email": fromEmail,
		},
		"subject": subject,
		"content": []map[string]string{
			{
				"type":  "text/html",
				"value": htmlBody,
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sendgrid marshal payload error: %w", err)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("sendgrid create request error: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid API call error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendgrid API failed [status %d]: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// 2. Amazon SES (Dùng AWS SDK v2)
func (m *MailConfig) sendViaAmazonSES(c *gin.Context, toEmail string, subject string, htmlBody string) error {
	accessKey, _ := GetPluginSetting(c, "aws_ses", "access_key_id")
	secretKey, _ := GetPluginSetting(c, "aws_ses", "secret_access_key")
	region, _ := GetPluginSetting(c, "aws_ses", "region")
	fromEmail, _ := GetPluginSetting(c, "aws_ses", "from_email")

	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("aws_ses credentials chưa được cấu hình đầy đủ trong plugin setting")
	}
	if region == "" {
		region = "us-east-1" // Fallback region mặc định
	}
	if fromEmail == "" {
		fromEmail = m.Sender
	}

	// Khởi tạo AWS Config động dựa trên keys lấy từ Plugin Setting
	cfg, err := config.LoadDefaultConfig(c.Request.Context(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return fmt.Errorf("không thể khởi tạo AWS config: %w", err)
	}

	sesClient := ses.NewFromConfig(cfg)

	// Build Email Request
	input := &ses.SendEmailInput{
		Source: aws.String(fromEmail),
		Destination: &sestypes.Destination{
			ToAddresses: []string{toEmail},
		},
		Message: &sestypes.Message{
			Subject: &sestypes.Content{
				Data:    aws.String(subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &sestypes.Body{
				Html: &sestypes.Content{
					Data:    aws.String(htmlBody),
					Charset: aws.String("UTF-8"),
				},
			},
		},
	}

	_, err = sesClient.SendEmail(c.Request.Context(), input)
	if err != nil {
		return fmt.Errorf("aws_ses send email error: %w", err)
	}

	return nil
}

// 3. Resend API
func (m *MailConfig) sendViaResend(c *gin.Context, toEmail string, subject string, htmlBody string) error {
	apiKey, _ := GetPluginSetting(c, "resend", "api_key")
	if apiKey == "" {
		return fmt.Errorf("resend api_key chưa được cấu hình trong plugin setting")
	}

	fromEmail, _ := GetPluginSetting(c, "resend", "from_email")
	if fromEmail == "" {
		fromEmail = m.Sender
	}

	payload := map[string]any{
		"from":    fromEmail,
		"to":      []string{toEmail},
		"subject": subject,
		"html":    htmlBody,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("resend marshal payload error: %w", err)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), "POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("resend create request error: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend API call error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API failed [status %d]: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// 4. SMTP truyền thống
func (m *MailConfig) sendViaSMTP(toEmail string, subject string, htmlBody string) error {
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subjectHeader := fmt.Sprintf("Subject: %s\n", subject)
	msg := []byte(subjectHeader + mime + htmlBody)

	auth := smtp.PlainAuth("", m.Sender, m.Password, m.SmtpHost)
	addr := fmt.Sprintf("%s:%s", m.SmtpHost, m.SmtpPort)

	return smtp.SendMail(addr, auth, m.Sender, []string{toEmail}, msg)
}
