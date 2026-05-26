package ws

import (
	"encoding/json"
	"sync"
	"time"

	"go-lobby/internal/model"

	"github.com/gorilla/websocket"
)

type MapClient struct {
	Hub    *MapHub
	UserID int64
	Conn   *websocket.Conn
	Send   chan []byte
	once   sync.Once
}

type SubscribeChunksMessage struct {
	ChunkIDs []string `json:"chunk_ids"`
}

func NewMapClient(hub *MapHub, userID int64, conn *websocket.Conn) *MapClient {
	return &MapClient{
		Hub:    hub,
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}
}

func (c *MapClient) ReadPump() {
	defer func() {
		c.Hub.Leave(c)
		c.Conn.Close()
	}()
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.SendError("json格式错误")
			continue
		}
		switch msg.Type {
		case "ping":
			c.SendJSON(ServerMessage{Type: "pong"})
		case "subscribe_chunks":
			c.handleSubscribeChunks(msg.Data)
		default:
			c.SendError("未知的消息类型")
		}
	}
}

func (c *MapClient) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *MapClient) handleSubscribeChunks(data json.RawMessage) {
	var msg SubscribeChunksMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.SendError("订阅参数错误")
		return
	}
	chunkIDs := make([]string, 0, len(msg.ChunkIDs))
	seen := make(map[string]bool, len(msg.ChunkIDs))
	for _, rawChunkID := range msg.ChunkIDs {
		chunkID, err := model.ParseChunkID(rawChunkID)
		if err != nil || model.EnsurePlayableChunk(chunkID) != nil {
			c.SendError("chunk_id 参数错误")
			return
		}
		if seen[rawChunkID] {
			continue
		}
		seen[rawChunkID] = true
		chunkIDs = append(chunkIDs, rawChunkID)
	}
	c.Hub.ReplaceSubscriptions(c, chunkIDs)
	c.SendJSON(ServerMessage{
		Type: "subscribed_chunks",
		Data: SubscribeChunksMessage{ChunkIDs: chunkIDs},
	})
}

func (c *MapClient) SendJSON(msg ServerMessage) {
	data, _ := EncodeServerMessage(msg)
	c.Send <- data
}

func (c *MapClient) SendError(code string) {
	msg := ServerMessage{
		Type:  "error",
		Error: code,
	}
	c.SendJSON(msg)
}

func (c *MapClient) closeSend() {
	c.once.Do(func() {
		close(c.Send)
	})
}
