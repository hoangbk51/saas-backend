package websocket

type ClientMessage struct {
	Type           string `json:"type"`
	ConversationID int64  `json:"conversation_id"`
	Content        string `json:"content,omitempty"`
}

type ServerMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}
