package services

import (
	"fmt"
	"strings"

	"go-saas/models"

	"github.com/jmoiron/sqlx"
)

func SendMessage(
	db *sqlx.DB,
	conversationID int64,
	role string,
	customerID int64,
	adminID int64,
	isSuperAdmin bool,
	guestToken string,
	messageType string,
	content string,
) (*models.Message, error) {

	content = strings.TrimSpace(content)

	if content == "" {
		return nil, fmt.Errorf(
			"message content is required",
		)
	}

	if len(content) > 10000 {
		return nil, fmt.Errorf(
			"message content is too long",
		)
	}

	if messageType == "" {
		messageType = "text"
	}

	err := ValidateChatAccess(
		db,
		conversationID,
		role,
		customerID,
		adminID,
		isSuperAdmin,
		guestToken,
	)

	if err != nil {
		return nil, err
	}

	senderType, err := GetSenderType(role)

	if err != nil {
		return nil, err
	}

	senderID := int64(0)

	switch role {

	case "customer":
		senderID = customerID

	case "admin":
		senderID = adminID
	}

	tx, err := db.Beginx()

	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	result, err := tx.Exec(
		`
		INSERT INTO messages (
			conversation_id,
			sender_type,
			sender_id,
			message_type,
			content
		)
		VALUES (?, ?, ?, ?, ?)
		`,
		conversationID,
		senderType,
		senderID,
		messageType,
		content,
	)

	if err != nil {
		return nil, err
	}

	messageID, err := result.LastInsertId()

	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(
		`
		UPDATE conversations
		SET
			last_message_at = NOW(),
			updated_at = NOW()
		WHERE id = ?
		`,
		conversationID,
	)

	if err != nil {
		return nil, err
	}

	var message models.Message

	err = tx.Get(
		&message,
		`
		SELECT
			id,
			conversation_id,
			sender_type,
			sender_id,
			message_type,
			content,
			created_at
		FROM messages
		WHERE id = ?
		LIMIT 1
		`,
		messageID,
	)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &message, nil
}
