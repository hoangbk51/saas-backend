package controllers

import (
	"net/http"
	"strconv"

	"go-saas/services"
	"go-saas/utils"

	"github.com/gin-gonic/gin"
)

func GetConversations(c *gin.Context) {
	role := c.GetString("chatRole")

	if role != "admin" {
		c.JSON(
			http.StatusForbidden,
			gin.H{
				"error": "admin access required",
			},
		)
		return
	}

	conversations, err := services.GetConversations(c)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"data": conversations,
		},
	)
}
func GetConversation(c *gin.Context) {
	conversationID, err := strconv.ParseInt(c.Param("conversationId"), 10, 64)

	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid conversation id",
			},
		)
		return
	}

	role := c.GetString("chatRole")

	customerID := utils.GetInt64FromContext(c, "customerID")

	adminID := utils.GetInt64FromContext(c, "adminID")

	isSuperAdmin := false

	if value, exists := c.Get("isSuperAdmin"); exists {
		if v, ok := value.(bool); ok {
			isSuperAdmin = v
		}
	}

	guestToken := c.GetString("chatGuestToken")

	db, err := utils.GetDBFromContext(c)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "database connection error"},
		)
		return
	}

	err = services.ValidateChatAccess(
		db,
		conversationID,
		role,
		customerID,
		adminID,
		isSuperAdmin,
		guestToken,
	)

	if err != nil {

	}

	conversation, err := services.GetConversation(
		c,
		conversationID,
	)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	if conversation == nil {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"error": "conversation not found",
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"data": conversation,
		},
	)
}

func CreateConversation(c *gin.Context) {
	customerID := utils.GetInt64FromContext(c, "customerID")

	result, err := services.CreateConversation(c, customerID)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"data": result,
		},
	)
}

func GetConversationMessages(c *gin.Context) {
	utils.LogToFile("GetConversationMessages")
	conversationID, err := strconv.ParseInt(
		c.Param("conversationId"),
		10,
		64,
	)

	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid conversation id",
			},
		)
		return
	}

	role := c.GetString("chatRole")

	if role == "" {
		role = "guest"
	}

	customerID := utils.GetInt64FromContext(
		c,
		"customerID",
	)

	adminID := utils.GetInt64FromContext(
		c,
		"adminID",
	)

	isSuperAdmin := false

	if value, exists := c.Get("isSuperAdmin"); exists {
		if v, ok := value.(bool); ok {
			isSuperAdmin = v
		}
	}

	// Ưu tiên X-Guest-Token
	guestToken := c.GetHeader("X-Guest-Token")

	// Fallback query ?token=
	if guestToken == "" {
		guestToken = c.Query("token")
	}

	utils.LogToFile(
		"[CHAT MESSAGES] conversationID=%d role=%s customerID=%d adminID=%d superAdmin=%v guestTokenPresent=%v",
		conversationID,
		role,
		customerID,
		adminID,
		isSuperAdmin,
		guestToken != "",
	)

	messages, err := services.GetConversationMessages(
		c,
		conversationID,
		role,
		customerID,
		adminID,
		isSuperAdmin,
		guestToken,
	)

	if err != nil {
		utils.LogToFile(
			"[ERROR ChatMessages] conversationID=%d role=%s error=%v",
			conversationID,
			role,
			err,
		)

		c.JSON(
			http.StatusForbidden,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		gin.H{
			"data": messages,
		},
	)
}
