package ws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
)

type Client struct {
	Hub    *RoomHub
	RoomID string
	UserID int64
	Conn   *websocket.Conn
	Send   chan []byte
}

func NewClient(hub *RoomHub, roomID string, userID int64, conn *websocket.Conn) *Client {
	return &Client{
		Hub:    hub,
		RoomID: roomID,
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.LeaveRoom(c)
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
		case "chat":
			c.Hub.BroadcastToRoom(c.RoomID, data)
		default:
			c.SendError("未知的消息类型")
		}
	}
}

func (c *Client) WritePump() {
	ticket := time.NewTicker(pingPeriod)
	defer func() {
		ticket.Stop()
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
		case <-ticket.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) SendJSON(msg ServerMessage) {
	data, _ := EncodeServerMessage(msg)
	c.Send <- data
}

func (c *Client) SendError(code string) {
	msg := ServerMessage{
		Type:  "error",
		Error: code,
	}
	c.SendJSON(msg)
}
