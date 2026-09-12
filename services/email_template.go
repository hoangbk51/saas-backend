package services

import (
	"fmt"
	"go-saas/models"
	"go-saas/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type EmailTemplateService struct{}

func NewEmailTemplateService() *EmailTemplateService {
	return &EmailTemplateService{}
}

// 1. Get All Templates
func (s *EmailTemplateService) GetAll(c *gin.Context, langCode string) ([]models.EmailTemplate, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể lấy DB context: %w", err)
	}

	templates := []models.EmailTemplate{}
	query := `
		SELECT id, name, lang_code, sender_name, sender_email, subject, body, type, position, files, created_at, updated_at 
		FROM email_templates 
		WHERE deleted_at IS NULL
	`

	if langCode != "" {
		query += " AND lang_code = ?"
		err = tenantDB.Select(&templates, query, langCode)
	} else {
		err = tenantDB.Select(&templates, query)
	}

	if err != nil {
		return nil, fmt.Errorf("lỗi truy vấn email_templates: %w", err)
	}

	return templates, nil
}

// 2. Get By ID
func (s *EmailTemplateService) GetByID(c *gin.Context, id uint32) (*models.EmailTemplate, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, fmt.Errorf("không thể lấy DB context: %w", err)
	}

	var tmpl models.EmailTemplate
	query := `
		SELECT id, name, lang_code, sender_name, sender_email, subject, body, type, position, files, created_at, updated_at 
		FROM email_templates 
		WHERE id = ? AND deleted_at IS NULL 
		LIMIT 1
	`
	err = tenantDB.Get(&tmpl, query, id)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy email template ID %d: %w", id, err)
	}

	return &tmpl, nil
}

// 3. Create Template
func (s *EmailTemplateService) Create(c *gin.Context, req models.CreateEmailTemplateReq) (int64, error) {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return 0, fmt.Errorf("không thể lấy DB context: %w", err)
	}

	if req.LangCode == "" {
		req.LangCode = "vi"
	}
	if req.Type == "" {
		req.Type = "HTML"
	}
	if req.Position == "" {
		req.Position = "Content"
	}

	now := time.Now()
	query := `
		INSERT INTO email_templates (name, lang_code, sender_name, sender_email, subject, body, type, position, files, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	res, err := tenantDB.Exec(query,
		req.Name, req.LangCode, req.SenderName, req.SenderEmail,
		req.Subject, req.Body, req.Type, req.Position, req.Files,
		now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("lỗi thêm email template: %w", err)
	}

	lastID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return lastID, nil
}

// 4. Update Template
func (s *EmailTemplateService) Update(c *gin.Context, id uint32, req models.UpdateEmailTemplateReq) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("không thể lấy DB context: %w", err)
	}

	if req.LangCode == "" {
		req.LangCode = "vi"
	}

	now := time.Now()
	query := `
		UPDATE email_templates 
		SET name = ?, lang_code = ?, sender_name = ?, sender_email = ?, subject = ?, body = ?, type = ?, position = ?, files = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`

	res, err := tenantDB.Exec(query,
		req.Name, req.LangCode, req.SenderName, req.SenderEmail,
		req.Subject, req.Body, req.Type, req.Position, req.Files, now,
		id,
	)
	if err != nil {
		return fmt.Errorf("lỗi cập nhật email template: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("không tìm thấy bản ghi để cập nhật")
	}

	return nil
}

// 5. Delete Template (Soft Delete)
func (s *EmailTemplateService) Delete(c *gin.Context, id uint32) error {
	tenantDB, err := utils.GetDBFromContext(c)
	if err != nil {
		return fmt.Errorf("không thể lấy DB context: %w", err)
	}

	now := time.Now()
	query := `UPDATE email_templates SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`

	res, err := tenantDB.Exec(query, now, id)
	if err != nil {
		return fmt.Errorf("lỗi xóa email template: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("bản ghi không tồn tại hoặc đã bị xóa")
	}

	return nil
}
