package services

import (
	"database/sql"

	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func GetConversations(c *gin.Context) ([]models.Conversation, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			id,
			customer_id,
			assigned_admin_id,
			status,
			last_message_at,
			created_at,
			updated_at
		FROM conversations
		ORDER BY updated_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := make([]models.Conversation, 0)

	for rows.Next() {
		var conversation models.Conversation

		err := rows.Scan(
			&conversation.ID,
			&conversation.CustomerID,
			&conversation.AssignedAdminID,
			&conversation.Status,
			&conversation.LastMessageAt,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		conversations = append(
			conversations,
			conversation,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return conversations, nil
}

func GetConversation(
	c *gin.Context,
	id int64,
) (*models.Conversation, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			id,
			customer_id,
			assigned_admin_id,
			status,
			last_message_at,
			created_at,
			updated_at,
			channel,external_chat_id,customer_name
		FROM conversations
		WHERE id = ?
		LIMIT 1
	`

	var conversation models.Conversation

	err = db.QueryRow(
		query,
		id,
	).Scan(
		&conversation.ID,
		&conversation.CustomerID,
		&conversation.AssignedAdminID,
		&conversation.Status,
		&conversation.LastMessageAt,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
		&conversation.Channel,
		&conversation.ExternalChatId,
		&conversation.CustomerName,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &conversation, nil
}

func GetConversationMessages(
	c *gin.Context,
	conversationID int64,
	role string,
	customerID int64,
	adminID int64,
	isSuperAdmin bool,
	guestToken string,
) ([]models.Message, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	err = ValidateChatAccess(
		db,
		conversationID,
		role,
		customerID,
		adminID,
		isSuperAdmin,
		guestToken,
	)

	if err != nil {
		//	return nil, err
	}

	rows, err := db.Query(
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
		WHERE conversation_id = ?
		ORDER BY id ASC
		`,
		conversationID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]models.Message, 0)

	for rows.Next() {
		var message models.Message

		err := rows.Scan(
			&message.ID,
			&message.ConversationID,
			&message.SenderType,
			&message.SenderID,
			&message.MessageType,
			&message.Content,
			&message.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		messages = append(
			messages,
			message,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func CreateConversation(
	c *gin.Context,
	customerID int64,
) (*models.CreateConversationResponse, error) {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return nil, err
	}

	var guestToken string
	var guestTokenHash string

	if customerID == 0 {
		guestToken, err = utils.GenerateGuestToken()
		if err != nil {
			return nil, err
		}

		guestTokenHash = utils.HashGuestToken(
			guestToken,
		)
	}

	query := `
		INSERT INTO conversations (
			customer_id,
			status,
			guest_token_hash
		)
		VALUES (?, 'ai', NULLIF(?, ''))
	`

	result, err := db.ExecContext(
		c,
		query,
		customerID,
		guestTokenHash,
	)

	if err != nil {
		return nil, err
	}

	conversationID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	var conversation models.Conversation

	err = db.Get(
		&conversation,
		`
		SELECT
			id,
			customer_id,
			assigned_admin_id,
			status,
			last_message_at,
			created_at,
			updated_at
		FROM conversations
		WHERE id = ?
		LIMIT 1
		`,
		conversationID,
	)

	if err != nil {
		return nil, err
	}

	return &models.CreateConversationResponse{
		Conversation: conversation,
		GuestToken:   guestToken,
	}, nil
}
