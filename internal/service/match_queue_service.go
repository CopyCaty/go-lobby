package service

import (
	"context"
	"errors"
	"fmt"
	"go-lobby/internal/dto/req"
	"go-lobby/internal/dto/res"
	"go-lobby/internal/matchqueue"
	"strings"
	"sync"
	"time"
)

type MatchQueueService struct {
	mu     sync.Mutex
	queues map[string][]*matchqueue.QueueEntry
	users  map[int64]*matchqueue.QueueUserState
}

func NewMatchQueueService() *MatchQueueService {
	return &MatchQueueService{
		queues: make(map[string][]*matchqueue.QueueEntry),
		users:  make(map[int64]*matchqueue.QueueUserState),
	}
}

func (s *MatchQueueService) Join(ctx context.Context, userID int64, req *req.JoinMatchQueueRequest) (*res.JoinMatchQueueResponse, error) {
	if req == nil {
		return nil, errors.New("请求不能为空")
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		return nil, errors.New("mode 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if state, ok := s.users[userID]; ok {
		switch state.Status {
		case matchqueue.QueueStatusMatching:
			return s.buildJoinResponse(state), nil
		case matchqueue.QueueStatusMatched:
			return s.buildJoinResponse(state), nil
		case matchqueue.QueueStatusCancelled, matchqueue.QueueStatusInit:
			delete(s.users, userID)
		default:
			delete(s.users, userID)
		}
	}

	now := time.Now()
	ticketID := generateQueueTicketID()
	entry := &matchqueue.QueueEntry{
		UserID:      userID,
		Mode:        mode,
		EnqueueTime: now,
		TicketID:    ticketID,
	}

	s.users[userID] = &matchqueue.QueueUserState{
		UserID:    userID,
		Mode:      mode,
		Status:    matchqueue.QueueStatusMatching,
		TicketID:  ticketID,
		UpdatedAt: now,
	}
	s.queues[mode] = append(s.queues[mode], entry)
	s.TryMatch(mode)

	userState, ok := s.users[userID]
	if !ok {
		return nil, errors.New("队列状态异常")
	}
	return s.buildJoinResponse(userState), nil
}

func (s *MatchQueueService) Status(ctx context.Context, userID int64) (*res.StatusMatchQueueResponse, error) {

	status, ok := s.users[userID]
	if !ok {
		return nil, errors.New("匹配状态不存在")
	}
	return &res.StatusMatchQueueResponse{
		UserID:    userID,
		Mode:      status.Mode,
		Status:    status.Status,
		TicketID:  status.TicketID,
		RoomID:    status.RoomID,
		Teams:     status.Teams,
		UpdatedAt: status.UpdatedAt,
	}, nil

}

func (s *MatchQueueService) Cancel(ctx context.Context, userID int64) (*res.StatusMatchQueueResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.users[userID]
	if !ok {
		return nil, errors.New("匹配状态不存在")
	}
	if status.Status != matchqueue.QueueStatusMatching {
		return nil, errors.New("当前状态不允许取消")
	}

	mode := status.Mode
	s.removeFromQueue(mode, userID)
	status.Status = matchqueue.QueueStatusCancelled
	status.UpdatedAt = time.Now()
	return &res.StatusMatchQueueResponse{
		UserID:    userID,
		Mode:      status.Mode,
		Status:    status.Status,
		TicketID:  status.TicketID,
		RoomID:    status.RoomID,
		Teams:     status.Teams,
		UpdatedAt: status.UpdatedAt,
	}, nil
}

func (s *MatchQueueService) buildJoinResponse(state *matchqueue.QueueUserState) *res.JoinMatchQueueResponse {
	return &res.JoinMatchQueueResponse{
		QueueStatus:   string(state.Status),
		QueueTicketID: state.TicketID,
		Mode:          state.Mode,
		RoomID:        state.RoomID,
		Teams:         state.Teams,
	}
}

func (s *MatchQueueService) TryMatch(mode string) {
	queue := s.queues[mode]
	required := getPlayerCount(mode)
	if required <= 0 {
		return
	}
	if len(queue) < int(required) {
		return
	}
	matchedEntries := queue[:required]
	remainEntries := queue[required:]
	s.queues[mode] = remainEntries
	roomID := generateRoomID()
	now := time.Now()
	teams := buildTeams(mode, matchedEntries)
	for _, entry := range matchedEntries {
		state, ok := s.users[entry.UserID]
		if !ok {
			continue
		}
		state.Status = matchqueue.QueueStatusMatched
		state.RoomID = roomID
		state.Teams = teams
		state.UpdatedAt = now
	}
}

func (s *MatchQueueService) removeFromQueue(mode string, userID int64) {
	queue := s.queues[mode]
	for i, entry := range queue {
		if entry.UserID == userID {
			s.queues[mode] = append(queue[:i], queue[i+1:]...)
			break
		}
	}
}

func getPlayerCount(mode string) int8 {
	switch mode {
	case "1v1":
		return 2
	case "2v2":
		return 4
	default:
		return 0
	}
}

func generateQueueTicketID() string {
	return fmt.Sprintf("qt_%d", time.Now().UnixNano())
}

func generateRoomID() string {
	return fmt.Sprintf("r_%d", time.Now().UnixNano())
}

func buildTeams(mode string, entries []*matchqueue.QueueEntry) []matchqueue.MatchedTeam {
	switch mode {
	case "1v1":
		return []matchqueue.MatchedTeam{
			{
				TeamID:  0,
				UserIDs: []int64{entries[0].UserID},
			},
			{
				TeamID:  1,
				UserIDs: []int64{entries[1].UserID},
			},
		}
	case "2v2":
		return []matchqueue.MatchedTeam{
			{
				TeamID:  0,
				UserIDs: []int64{entries[0].UserID, entries[1].UserID},
			},
			{
				TeamID:  1,
				UserIDs: []int64{entries[2].UserID, entries[3].UserID},
			},
		}
	default:
		return nil
	}
}
