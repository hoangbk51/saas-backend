package models

import (
	"fmt"
	"time"
)

type Address struct {
	ID              uint64    `json:"id"`
	Type            int       `json:"type"`
	AddressTitle    string    `json:"address_title"`
	AddressLine1    string    `json:"address_line_1"`
	AddressLine2    string    `json:"address_line_2"`
	City            string    `json:"city"`
	StateID         int       `json:"state_id"`
	ZipCode         string    `json:"zip_code"`
	CountryID       int       `json:"country_id"`
	Phone           string    `json:"phone"`
	Latitude        float64   `json:"latitude"`
	Longitude       float64   `json:"longitude"`
	AddressableID   uint64    `json:"addressable_id"`
	AddressableType string    `json:"addressable_type"`
	IsDefault       bool      `json:"is_default"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AddressPayload struct {
	Type            int     `json:"type"`
	AddressTitle    string  `json:"address_title" `
	AddressLine1    string  `json:"address_line_1" `
	AddressLine2    string  `json:"address_line_2"`
	City            string  `json:"city"`
	StateID         int     `json:"state_id"`
	ZipCode         string  `json:"zip_code"`
	CountryID       int     `json:"country_id"` // Tự convert "704" -> 704
	Phone           string  `json:"phone"`
	AddressableID   uint64  `json:"addressable_id"`
	AddressableType string  `json:"addressable_type"`
	IsDefault       BitBool `json:"is_default"` // Thay bool bằng BitBool
}

type BitBool bool

// Tự định nghĩa cách đọc JSON cho BitBool
func (bit *BitBool) UnmarshalJSON(data []byte) error {
	asString := string(data)

	switch asString {
	// Các trường hợp trả về True
	case "1", "true", "\"1\"", "\"true\"":
		*bit = true

	// Các trường hợp trả về False
	case "0", "false", "\"0\"", "\"false\"", "null", "\"\"":
		*bit = false

	// Trường hợp không xác định
	default:
		return fmt.Errorf("không thể convert giá trị '%s' sang boolean", asString)
	}

	return nil
}
