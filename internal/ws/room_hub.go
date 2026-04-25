package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

type RoomHub struct {
	mu    sync.RWMutex
	rooms map[string]map[int64]*websocket.Conn
}

func NewRoomHub() *RoomHub {
	return &RoomHub{
		rooms: make(map[string]map[int64]*websocket.Conn),
	}
}

func (h *RoomHub) JoinRoom(roomID string, userID int64, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[int64]*websocket.Conn)
	}
	h.rooms[roomID][userID] = conn
}

func (h *RoomHub) LeaveRoom(roomID string, userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] != nil {
		delete(h.rooms[roomID], userID)
		if len(h.rooms[roomID]) == 0 {
			delete(h.rooms, roomID)
		}
	}
}

func (h *RoomHub) BroadcastToRoom(roomID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.rooms[roomID] != nil {
		for _, conn := range h.rooms[roomID] {
			conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}
