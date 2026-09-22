package models

// --- CURRENCIES ---
type CurrencyInput struct {
	ISOCode            string   `json:"iso_code" binding:"required"`
	Name               string   `json:"name" binding:"required"`
	Symbol             string   `json:"symbol" binding:"required"`
	Priority           *int     `json:"priority"`
	SymbolFirst        *bool    `json:"symbol_first"`
	DecimalMark        *string  `json:"decimal_mark"`
	ThousandsSeparator *string  `json:"thousands_separator"`
	Active             *bool    `json:"active"`
	ExchangeRate       *float64 `json:"exchange_rate"`
}
