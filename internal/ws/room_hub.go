package ws

import (
	"sync"
)

type RoomHub struct {
	mu    sync.RWMutex
	rooms map[string]map[int64]*Client
}

func NewRoomHub() *RoomHub {
	return &RoomHub{
		rooms: make(map[string]map[int64]*Client),
	}
}

func (h *RoomHub) JoinRoom(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[client.RoomID] == nil {
		h.rooms[client.RoomID] = make(map[int64]*Client)
	}
	h.rooms[client.RoomID][client.UserID] = client
}

func (h *RoomHub) LeaveRoom(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[client.RoomID]
	if room == nil {
		return
	}
	if h.rooms[client.RoomID] != nil {
		if room[client.UserID] == client {
			delete(room, client.UserID)
			close(client.Send)
		}
		if len(h.rooms[client.RoomID]) == 0 {
			delete(h.rooms, client.RoomID)
		}
	}
}

func (h *RoomHub) BroadcastToRoom(roomID string, message []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[roomID]
	if room == nil {
		return
	}
	for _, client := range room {
		select {
		case client.Send <- message:
		default:
			delete(room, client.UserID)
			close(client.Send)
		}
	}
	if len(room) == 0 {
		delete(h.rooms, roomID)
	}
}
