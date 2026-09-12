package models

import "time"

type Message struct {
	ID             int64     `json:"id" db:"id"`
	ConversationID int64     `json:"conversation_id" db:"conversation_id"`
	SenderType     string    `json:"sender_type" db:"sender_type"`
	SenderID       int64     `json:"sender_id" db:"sender_id"`
	MessageType    string    `json:"message_type" db:"message_type"`
	Content        string    `json:"content" db:"content"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}
