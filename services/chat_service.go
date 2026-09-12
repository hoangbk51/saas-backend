package services

import (
	"fmt"

	"go-saas/utils"

	"github.com/jmoiron/sqlx"
)

func GetSenderType(role string) (string, error) {
	switch role {
	case "guest":
		return "guest", nil

	case "customer":
		return "customer", nil

	case "admin":
		return "admin", nil

	case "ai":
		return "ai", nil

	default:
		return "", fmt.Errorf("role cannot send chat message")
	}
}

func ValidateChatAccess(
	db *sqlx.DB,
	conversationID int64,
	role string,
	customerID int64,
	adminID int64,
	isSuperAdmin bool,
	guestToken string,
) error {

	var (
		conversationCustomerID int64
		assignedAdminID        *int64
		guestTokenHash         *string
	)

	err := db.QueryRow(
		`
		SELECT
			customer_id,
			assigned_admin_id,
			guest_token_hash
		FROM conversations
		WHERE id = ?
		LIMIT 1
		`,
		conversationID,
	).Scan(
		&conversationCustomerID,
		&assignedAdminID,
		&guestTokenHash,
	)

	if err != nil {
		return fmt.Errorf("conversation not found")
	}

	switch role {

	case "guest":

		if guestToken == "" {
			return fmt.Errorf("guest token is required")
		}

		if guestTokenHash == nil ||
			*guestTokenHash == "" {
			return fmt.Errorf("invalid guest access")
		}

		hash := utils.HashGuestToken(guestToken)

		if hash != *guestTokenHash {
			return fmt.Errorf("invalid guest token")
		}

		return nil

	case "customer":

		if customerID <= 0 {
			return fmt.Errorf("invalid customer")
		}

		if conversationCustomerID != customerID {
			return fmt.Errorf("conversation access denied")
		}

		return nil

	case "admin":

		if adminID <= 0 {
			return fmt.Errorf("invalid admin")
		}

		if isSuperAdmin {
			return nil
		}

		if assignedAdminID == nil {
			return nil
		}

		if *assignedAdminID != adminID {
			return fmt.Errorf("conversation assigned to another admin")
		}

		return nil

	default:
		return fmt.Errorf("invalid chat role")
	}
}
