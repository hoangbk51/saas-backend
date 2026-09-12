package models

type CreateZoneReq struct {
	Name        string           `json:"name" form:"name" binding:"required"`
	Description string           `json:"description" form:"description"`
	Details     []ZoneDetailItem `json:"details"`
}

type UpdateZoneReq struct {
	Name        string           `json:"name" form:"name" binding:"required"`
	Description string           `json:"description" form:"description"`
	Details     []ZoneDetailItem `json:"details"`
}

type ZoneDetailItem struct {
	CountryID int `json:"country_id"`
	StateID   int `json:"state_id"`
}
