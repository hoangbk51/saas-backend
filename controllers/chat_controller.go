package controllers

import (
	"net/http"
	"strconv"

	"go-saas/services"
	chatws "go-saas/websocket"

	"go-saas/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ChatWebSocket(c *gin.Context) {
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
		c.JSON(
			http.StatusForbidden,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	conn, err := upgrader.Upgrade(
		c.Writer,
		c.Request,
		nil,
	)

	if err != nil {
		return
	}

	client := &chatws.Client{
		Conn: conn,

		Send: make(chan []byte, 256),

		ConversationID: conversationID,

		Hub:  chatws.GlobalHub,
		Role: role,

		CustomerID: customerID,

		AdminID: adminID,

		IsSuperAdmin: isSuperAdmin,

		GuestToken: guestToken,

		DB: db,
	}

	chatws.GlobalHub.Join(

		conversationID,
		client,
	)

	go client.WritePump()

	client.ReadPump()
}
