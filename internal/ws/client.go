package ws

import (
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
		c.Hub.BroadcastToRoom(c.RoomID, data)
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
