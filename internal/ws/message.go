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

type ChunkEvent struct {
	Type    string      `json:"type"`
	ChunkID string      `json:"chunk_id"`
	Version int64       `json:"version,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func EncodeServerMessage(msg ServerMessage) ([]byte, error) {
	return json.Marshal(msg)
}

func EncodeChunkEvent(event ChunkEvent) ([]byte, error) {
	return json.Marshal(event)
}
