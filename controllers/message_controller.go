package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-saas/services"
	"go-saas/utils"
	chatws "go-saas/websocket"

	"github.com/gin-gonic/gin"
)

func SendMessage(c *gin.Context) {
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

	var request struct {
		MessageType string `json:"message_type"`
		Content     string `json:"content"`
	}

	if err := c.ShouldBindJSON(
		&request,
	); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid request body",
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

	if value, exists := c.Get(
		"isSuperAdmin",
	); exists {
		if v, ok := value.(bool); ok {
			isSuperAdmin = v
		}
	}

	guestToken := c.GetString("chatGuestToken")

	db, err := utils.GetDBFromContext(c)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": "database connection error",
			},
		)
		return
	}

	message, err := services.SendMessage(
		db,
		conversationID,
		role,
		customerID,
		adminID,
		isSuperAdmin,
		guestToken,
		request.MessageType,
		request.Content,
	)

	if err != nil {
		c.JSON(
			http.StatusForbidden,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	response, err := json.Marshal(
		chatws.OutgoingMessage{
			Type: "message",
			Data: message,
		},
	)

	if err != nil {
		utils.LogToFile(
			"[CHAT WS] marshal message failed: %v",
			err,
		)
	} else {
		chatws.GlobalHub.Broadcast(
			conversationID,
			response,
		)
	}

	c.JSON(
		http.StatusCreated,
		gin.H{
			"data": message,
		},
	)
}
