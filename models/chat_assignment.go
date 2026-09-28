package models

import "time"

// ChatAgent represents a support agent in the multi-agent distribution system
type ChatAgent struct {
	ID                       uint64     `json:"id" db:"id"`
	UserID                   uint64     `json:"user_id" db:"user_id"`
	Name                     string     `json:"name" db:"name"`
	Email                    *string    `json:"email" db:"email"`
	RoleTitle                string     `json:"role_title" db:"role_title"`
	Department               string     `json:"department" db:"department"`
	IsOnline                 bool       `json:"is_online" db:"is_online"`
	Status                   string     `json:"status" db:"status"` // 'online', 'away', 'offline'
	MaxCapacity              int        `json:"max_capacity" db:"max_capacity"`
	ActiveConversationsCount int        `json:"active_conversations_count" db:"active_conversations_count"`
	LastAssignedAt           *time.Time `json:"last_assigned_at" db:"last_assigned_at"`
	CreatedAt                time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at" db:"updated_at"`
}

// ChatAssignmentSettings represents the configuration for conversation assignment
type ChatAssignmentSettings struct {
	ID                          uint64    `json:"id" db:"id"`
	AutoAssignmentEnabled       bool      `json:"auto_assignment_enabled" db:"auto_assignment_enabled"`
	Strategy                    string    `json:"strategy" db:"strategy"` // 'round_robin', 'workload'
	DefaultMaxCapacity          int       `json:"default_max_capacity" db:"default_max_capacity"`
	FallbackAction              string    `json:"fallback_action" db:"fallback_action"` // 'queue', 'ai_fallback'
	NotifyOnTransfer            bool      `json:"notify_on_transfer" db:"notify_on_transfer"`
	ReassignOnInactivityMinutes int       `json:"reassign_on_inactivity_minutes" db:"reassign_on_inactivity_minutes"`
	UpdatedAt                   time.Time `json:"updated_at" db:"updated_at"`
}

// ChatTransfer represents a transfer record when a chat is reassigned
type ChatTransfer struct {
	ID             uint64    `json:"id" db:"id"`
	ConversationID uint64    `json:"conversation_id" db:"conversation_id"`
	FromAdminID    *uint64   `json:"from_admin_id" db:"from_admin_id"`
	FromAdminName  *string   `json:"from_admin_name" db:"from_admin_name"`
	ToAdminID      uint64    `json:"to_admin_id" db:"to_admin_id"`
	ToAdminName    *string   `json:"to_admin_name" db:"to_admin_name"`
	Reason         *string   `json:"reason" db:"reason"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// AgentKpiMetric represents performance and KPI metrics for each support staff
type AgentKpiMetric struct {
	AgentID                 uint64  `json:"agent_id"`
	UserID                  uint64  `json:"user_id"`
	Name                    string  `json:"name"`
	Email                   string  `json:"email"`
	RoleTitle               string  `json:"role_title"`
	Department              string  `json:"department"`
	Status                  string  `json:"status"`
	IsOnline                bool    `json:"is_online"`
	MaxCapacity             int     `json:"max_capacity"`
	CurrentActive           int     `json:"current_active"`
	TotalAssigned           int     `json:"total_assigned"`
	TotalClosed             int     `json:"total_closed"`
	AvgFirstResponseSeconds int     `json:"avg_first_response_seconds"`
	AvgResponseDisplay      string  `json:"avg_response_display"`
	TotalOrdersClosed       int     `json:"total_orders_closed"`
	ConversionRate          int     `json:"conversion_rate"`
	TotalRevenue            float64 `json:"total_revenue"`
	TotalRevenueDisplay     string  `json:"total_revenue_display"`
	Rating                  float64 `json:"rating"`
}

// KpiSummary represents the overall overview of all staff KPI metrics
type KpiSummary struct {
	TotalAgents               int     `json:"total_agents"`
	OnlineAgents              int     `json:"online_agents"`
	TotalConversationsHandled int     `json:"total_conversations_handled"`
	TotalOrdersClosed         int     `json:"total_orders_closed"`
	TotalRevenue              float64 `json:"total_revenue"`
	TotalRevenueDisplay       string  `json:"total_revenue_display"`
	AvgConversionRate         int     `json:"avg_conversion_rate"`
	OverallFrtDisplay         string  `json:"overall_frt_display"`
}

// Request payloads
type UpdateSettingsRequest struct {
	AutoAssignmentEnabled       *bool   `json:"auto_assignment_enabled"`
	Strategy                    *string `json:"strategy"`
	DefaultMaxCapacity          *int    `json:"default_max_capacity"`
	FallbackAction              *string `json:"fallback_action"`
	NotifyOnTransfer            *bool   `json:"notify_on_transfer"`
	ReassignOnInactivityMinutes *int    `json:"reassign_on_inactivity_minutes"`
}

type UpdateAgentRequest struct {
	Status      *string `json:"status"`
	IsOnline    *bool   `json:"is_online"`
	MaxCapacity *int    `json:"max_capacity"`
	Department  *string `json:"department"`
	RoleTitle   *string `json:"role_title"`
}

type AutoAssignRequest struct {
	ConversationID uint64  `json:"conversation_id" binding:"required"`
	AdminID        *uint64 `json:"admin_id"`
	AdminName      *string `json:"admin_name"`
}

type TransferChatRequest struct {
	ConversationID uint64  `json:"conversation_id" binding:"required"`
	FromAdminID    *uint64 `json:"from_admin_id"`
	FromAdminName  *string `json:"from_admin_name"`
	ToAdminID      uint64  `json:"to_admin_id" binding:"required"`
	ToAdminName    *string `json:"to_admin_name"`
	Reason         *string `json:"reason"`
}

type InternalNoteRequest struct {
	ConversationID uint64 `json:"conversation_id" binding:"required"`
	AdminID        uint64 `json:"admin_id"`
	AdminName      string `json:"admin_name"`
	Note           string `json:"note" binding:"required"`
}
