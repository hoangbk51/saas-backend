package models

// --- LANGUAGES ---
type LanguageInput struct {
	Code          string `json:"code" binding:"required"`
	PhpLocaleCode string `json:"php_locale_code" binding:"required"`
	Language      string `json:"language" binding:"required"` // Tên ngôn ngữ
	Order         *int   `json:"order"`
	RTL           *bool  `json:"rtl"`
	Active        *bool  `json:"active"`
	IsBackend     *bool  `json:"is_backend"`
	IsFrontend    *bool  `json:"is_frontend"`
}
