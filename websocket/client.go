package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jmoiron/sqlx"
)

const (
	writeWait = 10 * time.Second

	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10

	maxMessageSize = 10000
)

type Client struct {
	Conn *websocket.Conn

	Send chan []byte

	ConversationID int64

	Hub *Hub

	Role string

	CustomerID int64

	AdminID int64

	IsSuperAdmin bool

	GuestToken string

	DB *sqlx.DB

	closeOnce sync.Once
}

type IncomingMessage struct {
	MessageType string `json:"message_type"`

	Content string `json:"content"`
}

type OutgoingMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func (client *Client) ReadPump() {
	defer func() {
		client.Hub.Leave(client.ConversationID, client)
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(maxMessageSize)

	client.Conn.SetReadDeadline(
		time.Now().Add(pongWait),
	)

	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(
			time.Now().Add(pongWait),
		)
		return nil
	})

	for {
		if _, _, err := client.Conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (client *Client) WritePump() {
	ticker := time.NewTicker(
		pingPeriod,
	)

	defer func() {
		ticker.Stop()

		client.Conn.Close()
	}()

	for {
		select {

		case message, ok := <-client.Send:

			client.Conn.SetWriteDeadline(
				time.Now().Add(writeWait),
			)

			if !ok {
				client.Conn.WriteMessage(
					websocket.CloseMessage,
					[]byte{},
				)

				return
			}

			err := client.Conn.WriteMessage(
				websocket.TextMessage,
				message,
			)

			if err != nil {
				return
			}

		case <-ticker.C:

			client.Conn.SetWriteDeadline(
				time.Now().Add(writeWait),
			)

			if err := client.Conn.WriteMessage(
				websocket.PingMessage,
				nil,
			); err != nil {
				return
			}
		}
	}
}

func (client *Client) sendError(
	message string,
) {
	response, err := json.Marshal(
		OutgoingMessage{
			Type:  "error",
			Error: message,
		},
	)

	if err != nil {
		return
	}

	select {
	case client.Send <- response:

	default:
	}
}

func (client *Client) Close() {
	client.closeOnce.Do(func() {
		close(client.Send)
	})
}
