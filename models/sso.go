package models

import "time"

type SSOExchangeRequest struct {
	Code string `json:"code" binding:"required"`
}

type SSOTenantInfo struct {
	TenantID  string `json:"tenant_id"`
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain"`
}

type SSOUser struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	FullName     string `json:"full_name"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

type SSOExchangeData struct {
	AccessToken string        `json:"access_token"`
	TokenType   string        `json:"token_type"`
	ExpiresAt   string        `json:"expires_at"`
	TenantInfo  SSOTenantInfo `json:"tenant_info"`
	User        SSOUser       `json:"user"`
}

type SSOExchangeResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    SSOExchangeData `json:"data"`
}

type SSOCodeModel struct {
	ID        int64
	Code      string
	ClientID  int64
	TenantID  string
	Used      bool
	ExpiresAt time.Time
}
