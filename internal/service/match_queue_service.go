package service

import (
	"context"
	"errors"
	"fmt"
	"go-lobby/internal/dto/req"
	"go-lobby/internal/dto/res"
	"go-lobby/internal/matchqueue"
	"go-lobby/internal/repository"
	"strings"
	"sync"
	"time"
)

type MatchQueueService struct {
	mu   sync.Mutex
	repo *repository.MatchQueueRepository
	rs   *RoomService
	ms   *MatchService
}

func NewMatchQueueService(ms *MatchService, rs *RoomService, repo *repository.MatchQueueRepository) *MatchQueueService {
	return &MatchQueueService{
		repo: repo,
		rs:   rs,
		ms:   ms,
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

	state, err := s.repo.GetUserStatus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户匹配状态失败: %w", err)
	}
	if state != nil {
		switch state.Status {
		case matchqueue.QueueStatusMatching, matchqueue.QueueStatusMatched:
			return s.buildJoinResponse(state), nil
		case matchqueue.QueueStatusCancelled, matchqueue.QueueStatusInit:
			if err := s.repo.DeleteUserStatus(ctx, userID); err != nil {
				return nil, fmt.Errorf("删除用户匹配状态失败: %w", err)
			}
		default:
			if err := s.repo.DeleteUserStatus(ctx, userID); err != nil {
				return nil, fmt.Errorf("删除用户匹配状态失败: %w", err)
			}
		}
	}
	now := time.Now()
	ticketID := generateQueueTicketID()

	state = &matchqueue.QueueUserState{
		UserID:      userID,
		Mode:        mode,
		Status:      matchqueue.QueueStatusMatching,
		TicketID:    ticketID,
		UpdatedAt:   now,
		EnqueueTime: now,
	}
	if err := s.repo.SetUserStatus(ctx, state); err != nil {
		return nil, fmt.Errorf("设置用户匹配状态失败: %w", err)
	}

	if err := s.repo.Enqueue(ctx, mode, userID); err != nil {
		return nil, fmt.Errorf("加入匹配队列失败: %w", err)
	}
	matchTeams, roomID, err := s.FindMatchGroup(ctx, mode)
	if err != nil {
		return nil, fmt.Errorf("查找匹配组失败: %w", err)
	}
	if matchTeams == nil {
		userState, err := s.repo.GetUserStatus(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("获取用户匹配状态失败: %w", err)
		}
		return s.buildJoinResponse(userState), nil
	}
	fmt.Println("Match Found!")

	matchID, err := s.ms.CreateMatchFromQueue(ctx, &matchqueue.MatchQueueResult{
		RoomID: roomID,
		Mode:   mode,
		Teams:  matchTeams,
	})
	if err != nil {
		fmt.Println("Create Match From Queue Failed!")
		if err := s.restoreMatchedUsers(ctx, matchTeams, mode); err != nil {
			return nil, fmt.Errorf("恢复匹配用户失败: %w", err)
		}
		return nil, errors.New("创建比赛失败")
	}
	if err := s.updateUserStateToMatched(ctx, matchTeams, roomID, matchID); err != nil {
		return nil, fmt.Errorf("更新用户匹配状态失败: %w", err)
	}
	if _, err := s.rs.CreateRoom(roomID, mode, matchID, matchTeams); err != nil {
		return nil, errors.New("创建房间失败")
	}
	userState, err := s.repo.GetUserStatus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户匹配状态失败: %w", err)
	}
	return s.buildJoinResponse(userState), nil
}

func (s *MatchQueueService) Status(ctx context.Context, userID int64) (*res.StatusMatchQueueResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, err := s.repo.GetUserStatus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户匹配状态失败: %w", err)
	}
	if status == nil {
		return nil, errors.New("匹配状态不存在")
	}
	return &res.StatusMatchQueueResponse{
		UserID:    userID,
		MatchID:   status.MatchID,
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
	status, err := s.repo.GetUserStatus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户匹配状态失败: %w", err)
	}
	if status == nil {
		return nil, errors.New("匹配状态不存在")
	}
	if status.Status != matchqueue.QueueStatusMatching {
		return nil, errors.New("当前状态不允许取消")
	}

	mode := status.Mode
	if err := s.repo.RemoveFromQueue(ctx, mode, userID); err != nil {
		return nil, fmt.Errorf("从匹配队列中移除用户失败: %w", err)
	}
	status.Status = matchqueue.QueueStatusCancelled
	status.UpdatedAt = time.Now()
	if err := s.repo.SetUserStatus(ctx, status); err != nil {
		return nil, fmt.Errorf("更新用户匹配状态失败: %w", err)
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

func (s *MatchQueueService) buildJoinResponse(state *matchqueue.QueueUserState) *res.JoinMatchQueueResponse {
	return &res.JoinMatchQueueResponse{
		QueueStatus:   string(state.Status),
		QueueTicketID: state.TicketID,
		MatchID:       state.MatchID,
		Mode:          state.Mode,
		RoomID:        state.RoomID,
		Teams:         state.Teams,
	}
}

func (s *MatchQueueService) FindMatchGroup(ctx context.Context, mode string) ([]matchqueue.MatchedTeam, string, error) {
	required := getPlayerCount(mode)
	if required <= 0 {
		return nil, "", fmt.Errorf("无效的匹配模式: %s", mode)
	}
	len, err := s.repo.QueueLen(ctx, mode)
	if err != nil {
		return nil, "", fmt.Errorf("获取队列长度失败: %w", err)
	}
	if int(len) < int(required) {
		return nil, "", nil
	}
	userIDs, err := s.repo.DequeueBatch(ctx, mode, int64(required))
	if err != nil {
		return nil, "", fmt.Errorf("从队列中批量弹出用户失败 for mode %s: %w", mode, err)
	}

	roomID := generateRoomID()
	teams := buildTeams(mode, userIDs)
	return teams, roomID, nil
}

func (s *MatchQueueService) updateUserStateToMatched(ctx context.Context, teams []matchqueue.MatchedTeam, roomID string, matchID int64) error {
	now := time.Now()
	for _, team := range teams {
		for _, userID := range team.UserIDs {
			state, err := s.repo.GetUserStatus(ctx, userID)
			if err != nil {
				return fmt.Errorf("获取用户匹配状态失败 for userID %d: %w", userID, err)
			}
			if state == nil {
				continue
			}
			state.Status = matchqueue.QueueStatusMatched
			state.RoomID = roomID
			state.MatchID = matchID
			state.Teams = teams
			state.UpdatedAt = now
			if err := s.repo.SetUserStatus(ctx, state); err != nil {
				return fmt.Errorf("更新用户匹配状态失败 for userID %d: %w", userID, err)
			}
		}
	}
	return nil
}

func (s *MatchQueueService) restoreMatchedUsers(ctx context.Context, teams []matchqueue.MatchedTeam, mode string) error {
	for _, team := range teams {
		for _, userID := range team.UserIDs {
			userState, err := s.repo.GetUserStatus(ctx, userID)
			if err != nil {
				return fmt.Errorf("获取用户匹配状态失败 for userID %d: %w", userID, err)
			}
			if userState == nil {
				continue
			}
			if err := s.repo.Enqueue(ctx, mode, userID); err != nil {
				return fmt.Errorf("将用户重新加入队列失败 for userID %d: %w", userID, err)
			}
		}
	}
	return nil
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

func buildTeams(mode string, userIDs []int64) []matchqueue.MatchedTeam {
	switch mode {
	case "1v1":
		return []matchqueue.MatchedTeam{
			{
				TeamID:  0,
				UserIDs: []int64{userIDs[0]},
			},
			{
				TeamID:  1,
				UserIDs: []int64{userIDs[1]},
			},
		}
	case "2v2":
		return []matchqueue.MatchedTeam{
			{
				TeamID:  0,
				UserIDs: []int64{userIDs[0], userIDs[1]},
			},
			{
				TeamID:  1,
				UserIDs: []int64{userIDs[2], userIDs[3]},
			},
		}
	default:
		return nil
	}
}
