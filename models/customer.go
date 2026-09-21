package models

type CustomerInput struct {
	Name             string  `json:"name" binding:"required"`
	NiceName         *string `json:"nice_name"`
	Email            string  `json:"email" binding:"required,email"`
	Password         *string `json:"password"`
	Phone            *string `json:"phone"`
	Dob              *string `json:"dob"`
	Sex              *string `json:"sex"`
	Description      *string `json:"description"`
	Active           *int    `json:"active"`
	AcceptsMarketing *int    `json:"accepts_marketing"`
}

type CustomerHistoryInput struct {
	Comment string `json:"comment" binding:"required"`
}
