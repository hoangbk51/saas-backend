package websocket

import (
	"fmt"
	"sync"
)

type Hub struct {
	rooms map[int64]map[*Client]bool
	mu    sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[int64]map[*Client]bool),
	}
}

var GlobalHub = NewHub()

func (h *Hub) Join(conversationID int64, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[conversationID] == nil {
		h.rooms[conversationID] = make(map[*Client]bool)
	}

	h.rooms[conversationID][client] = true

	fmt.Printf(
		"[CHAT WS] JOIN conversation=%d clients=%d role=%s customer=%d admin=%d\n",
		conversationID,
		len(h.rooms[conversationID]),
		client.Role,
		client.CustomerID,
		client.AdminID,
	)
}

func (h *Hub) Leave(conversationID int64, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, exists := h.rooms[conversationID]
	if !exists {
		return
	}

	delete(room, client)

	fmt.Printf(
		"[CHAT WS] LEAVE conversation=%d clients=%d role=%s\n",
		conversationID,
		len(room),
		client.Role,
	)

	if len(room) == 0 {
		delete(h.rooms, conversationID)
	}
}

func (h *Hub) Broadcast(conversationID int64, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room, exists := h.rooms[conversationID]

	if !exists {
		fmt.Printf(
			"[CHAT WS] BROADCAST conversation=%d NO ROOM\n",
			conversationID,
		)
		return
	}

	fmt.Printf(
		"[CHAT WS] BROADCAST conversation=%d clients=%d message=%s\n",
		conversationID,
		len(room),
		string(message),
	)

	for client := range room {
		select {
		case client.Send <- message:
			fmt.Printf(
				"[CHAT WS] SEND conversation=%d role=%s customer=%d admin=%d\n",
				conversationID,
				client.Role,
				client.CustomerID,
				client.AdminID,
			)

		default:
			fmt.Printf(
				"[CHAT WS] SEND BUFFER FULL conversation=%d\n",
				conversationID,
			)
			go client.Close()
		}
	}
}
