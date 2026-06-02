package ws

import (
	"hash/fnv"
	"sync"
)

const roomHubSegmentCount = 32

type roomHubSegment struct {
	mu    sync.RWMutex
	rooms map[string]map[int64]*Client
}

// RoomHub 按 roomID 哈希分段加锁，不同房间的 Join / Leave / Broadcast 可并发执行。
type RoomHub struct {
	segments [roomHubSegmentCount]roomHubSegment
}

func NewRoomHub() *RoomHub {
	h := &RoomHub{}
	for i := range h.segments {
		h.segments[i].rooms = make(map[string]map[int64]*Client)
	}
	return h
}

func roomHubSegmentIndex(roomID string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(roomID))
	return hasher.Sum32() % roomHubSegmentCount
}

func (h *RoomHub) roomSegment(roomID string) *roomHubSegment {
	return &h.segments[roomHubSegmentIndex(roomID)]
}

func (h *RoomHub) JoinRoom(client *Client) {
	seg := h.roomSegment(client.RoomID)
	seg.mu.Lock()
	defer seg.mu.Unlock()
	if seg.rooms[client.RoomID] == nil {
		seg.rooms[client.RoomID] = make(map[int64]*Client)
	}
	seg.rooms[client.RoomID][client.UserID] = client
}

func (h *RoomHub) LeaveRoom(client *Client) {
	seg := h.roomSegment(client.RoomID)
	seg.mu.Lock()
	defer seg.mu.Unlock()
	room := seg.rooms[client.RoomID]
	if room == nil {
		return
	}
	if seg.rooms[client.RoomID] != nil {
		if room[client.UserID] == client {
			delete(room, client.UserID)
			close(client.Send)
		}
		if len(seg.rooms[client.RoomID]) == 0 {
			delete(seg.rooms, client.RoomID)
		}
	}
}

func (h *RoomHub) BroadcastToRoom(roomID string, message []byte) {
	seg := h.roomSegment(roomID)
	seg.mu.Lock()
	defer seg.mu.Unlock()
	room := seg.rooms[roomID]
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
		delete(seg.rooms, roomID)
	}
}
