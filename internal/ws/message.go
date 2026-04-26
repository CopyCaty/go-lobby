package ws

import "encoding/json"

type ClientMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ServerMessage struct {
	Type   string      `json:"type"`
	RoomID string      `json:"room_id,omitempty"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func EncodeServerMessage(msg ServerMessage) ([]byte, error) {
	return json.Marshal(msg)
}
