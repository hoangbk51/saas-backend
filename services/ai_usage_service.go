package services

import (
	"go-saas/models"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func LogAIUsage(c *gin.Context, usage *models.AIUsage) error {
	db, err := utils.GetDBFromContext(c)
	if err != nil {
		return err
	}

	query := `INSERT INTO ai_usage_history 
              ( action_type,prompt_tokens, completion_tokens, total_tokens, created_at) 
              VALUES (?, ?, ?, ?, NOW())`

	args := []interface{}{
		usage.Type,
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.TotalTokens,
	}

	utils.LogSQL(query, args...)

	_, err = db.Exec(query, args...)
	return err
}
