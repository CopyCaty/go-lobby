package service

import (
	"fmt"
	"go-lobby/internal/dto/res"
	"go-lobby/internal/matchqueue"
	"go-lobby/internal/model"
	"sync"
	"time"
)

type RoomService struct {
	mu    sync.Mutex
	rooms map[string]*model.Room
}

func NewRoomService() *RoomService {
	return &RoomService{
		rooms: make(map[string]*model.Room),
	}
}

func (s *RoomService) CreateRoom(roomID string, mode string, matchID int64, teams []matchqueue.MatchedTeam) (*model.Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	players := make(map[int64]*model.RoomPlayer)
	for _, team := range teams {
		for _, userID := range team.UserIDs {
			players[userID] = &model.RoomPlayer{
				UserID:    userID,
				TeamNo:    team.TeamID,
				Online:    true,
				Ready:     false,
				UpdatedAt: time.Now(),
			}
		}
	}
	room := &model.Room{
		ID:      roomID,
		Mode:    mode,
		MatchID: matchID,
		Status:  model.RoomStatusWaiting,
		Teams:   teams,
		Players: players,
	}

	s.rooms[roomID] = room
	return room, nil
}

func (s *RoomService) GetRoom(roomID string, userID int64) (*res.RoomInfoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room, exists := s.rooms[roomID]
	if !exists {
		return nil, fmt.Errorf("room with ID %s not found", roomID)
	}
	if !s.checkUserInRoom(room, userID) {
		return nil, fmt.Errorf("user %d is not in room %s", userID, roomID)
	}
	return &res.RoomInfoResponse{
		ID:      room.ID,
		Mode:    room.Mode,
		MatchID: room.MatchID,
		Status:  room.Status,
		Teams:   room.Teams,
		Players: room.Players,
	}, nil
}

func (s *RoomService) checkUserInRoom(room *model.Room, userID int64) bool {
	_, exists := room.Players[userID]
	return exists
}

func (s *RoomService) ReadyPlayer(roomID string, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	room, exists := s.rooms[roomID]
	if !exists {
		return fmt.Errorf("room with ID %s not found", roomID)
	}
	if room.Status != model.RoomStatusWaiting {
		return fmt.Errorf("room %s is not in waiting status", roomID)
	}
	player, exists := room.Players[userID]
	if !exists {
		return fmt.Errorf("player with ID %d not found in room %s", userID, roomID)
	}
	player.Ready = true
	player.UpdatedAt = time.Now()

	r, err := s.CheckAllReady(roomID)
	if err != nil {
		return fmt.Errorf("failed to check ready status: %w", err)
	}
	if r {
		room.Status = model.RoomStatusPlaying
	}
	return nil
}

func (s *RoomService) CheckAllReady(roomID string) (bool, error) {
	room, exists := s.rooms[roomID]
	if !exists {
		return false, fmt.Errorf("room with ID %s not found", roomID)
	}
	for _, player := range room.Players {
		if !player.Ready {
			return false, nil
		}
	}
	return true, nil
}
