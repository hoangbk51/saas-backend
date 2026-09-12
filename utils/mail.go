package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

	"github.com/gin-gonic/gin"
)

// MailConfig chứa thông tin cấu hình Gmail
type MailConfig struct {
	Sender   string
	Password string // App Password
	SmtpHost string
	SmtpPort string
}

// Hàm khởi tạo MailConfig (nên lấy từ file .env)
func NewMailConfig() *MailConfig {
	return &MailConfig{
		Sender:   "phamhuyhoang1112@gmail.com",
		Password: "glcywkiljjgoexfu",
		SmtpHost: "smtp.gmail.com",
		SmtpPort: "587",
	}
}

// SendVerificationEmail là phương thức gửi mail xác thực
func (m *MailConfig) SendVerificationEmail(toEmail string, name string, token string, locale string) error {
	// 1. Dữ liệu đổ vào template
	data := struct {
		Name string
		Link string
	}{
		Name: name,
		Link: fmt.Sprintf("http://hoangk53.central.test/api/v2/verify?token=%s", token),
	}

	// 2. Xác định Subject theo ngôn ngữ (Có thể dùng map để quản lý tập trung)
	subjects := map[string]string{
		"vi": "Xác nhận tài khoản của bạn",
		"en": "Verify your account",
		"jp": "アカウントの xác nhận", // Ví dụ thêm tiếng Nhật
	}

	subject, ok := subjects[locale]
	if !ok {
		subject = subjects["en"] // Mặc định là tiếng Anh nếu không tìm thấy locale
	}

	// 3. Cộng chuỗi để lấy đường dẫn template
	// Kết quả: templates/emails/vi/verify_email.html
	templatePath := fmt.Sprintf("templates/emails/%s/verify_email.html", locale)

	// 4. Parse và Thực thi Template
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		// Nếu không tìm thấy template ngôn ngữ đó, fallback về "en"
		fallbackPath := "templates/emails/en/verify_email.html"
		tmpl, err = template.ParseFiles(fallbackPath)
		if err != nil {
			return fmt.Errorf("không tìm thấy cả template chính và fallback: %v", err)
		}
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("lỗi đổ dữ liệu vào template: %v", err)
	}

	// 5. Gửi Mail
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subjectHeader := fmt.Sprintf("Subject: %s\n", subject)
	msg := []byte(subjectHeader + mime + body.String())

	auth := smtp.PlainAuth("", m.Sender, m.Password, m.SmtpHost)
	addr := fmt.Sprintf("%s:%s", m.SmtpHost, m.SmtpPort)

	return smtp.SendMail(addr, auth, m.Sender, []string{toEmail}, msg)
}

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
		return fmt.Errorf(
			"không tìm thấy email template '%s': %w",
			templateName,
			err,
		)
	}

	// 2. Render Body bằng html/template (Xử lý được vòng lặp {{range}})
	tmpl, err := template.New("email").Parse(rawBody)
	if err != nil {
		return fmt.Errorf("lỗi parse template body: %v", err)
	}

	var bodyBuffer bytes.Buffer
	if err := tmpl.Execute(&bodyBuffer, data); err != nil {
		return fmt.Errorf("lỗi execute template: %v", err)
	}

	// 3. Render Subject (Thường subject chỉ là chuỗi đơn nên có thể dùng text/template hoặc replace đơn giản)
	// Ở đây dùng luôn template cho đồng bộ
	subTmpl, err := template.New("subject").Parse(rawSubject)
	if err != nil {
		return err
	}
	var subBuffer bytes.Buffer
	subTmpl.Execute(&subBuffer, data)

	// 4. Gửi Mail
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subjectHeader := fmt.Sprintf("Subject: %s\n", subBuffer.String())
	msg := []byte(subjectHeader + mime + bodyBuffer.String())

	auth := smtp.PlainAuth("", m.Sender, m.Password, m.SmtpHost)
	addr := fmt.Sprintf("%s:%s", m.SmtpHost, m.SmtpPort)

	return smtp.SendMail(addr, auth, m.Sender, []string{toEmail}, msg)
}
