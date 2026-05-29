package service

import (
	"context"
	"errors"
	"fmt"
	"go-lobby/internal/dto/req"
	"go-lobby/internal/dto/res"
	"go-lobby/internal/matchqueue"
	"go-lobby/internal/repository"
	"log"
	"strings"
	"sync"
	"time"
)

type MatchQueueService struct {
	mu               sync.Mutex
	repo             *repository.MatchQueueRepository
	rs               *RoomService
	ms               *MatchService
	mineMatchService *MineMatchService
}

func NewMatchQueueService(ms *MatchService, rs *RoomService, repo *repository.MatchQueueRepository, mineMatchService ...*MineMatchService) *MatchQueueService {
	var mms *MineMatchService
	if len(mineMatchService) > 0 {
		mms = mineMatchService[0]
	}
	return &MatchQueueService{
		repo:             repo,
		rs:               rs,
		ms:               ms,
		mineMatchService: mms,
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
	log.Printf("match.queue.join start: user_id=%d mode=%s", userID, mode)

	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.repo.GetUserStatus(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户匹配状态失败: %w", err)
	}
	if state != nil {
		log.Printf("match.queue.join existing state: user_id=%d mode=%s status=%s match_id=%d room_id=%s chunk_id=%s", userID, state.Mode, state.Status, state.MatchID, state.RoomID, state.ChunkID)
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
	log.Printf("match.queue.join state saved: user_id=%d mode=%s ticket_id=%s", userID, mode, ticketID)

	if err := s.repo.Enqueue(ctx, mode, userID); err != nil {
		return nil, fmt.Errorf("加入匹配队列失败: %w", err)
	}
	log.Printf("match.queue.join enqueued: user_id=%d mode=%s", userID, mode)
	matchTeams, roomID, err := s.FindMatchGroup(ctx, mode)
	if err != nil {
		return nil, fmt.Errorf("查找匹配组失败: %w", err)
	}
	if matchTeams == nil {
		log.Printf("match.queue.join waiting: user_id=%d mode=%s", userID, mode)
		userState, err := s.repo.GetUserStatus(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("获取用户匹配状态失败: %w", err)
		}
		return s.buildJoinResponse(userState), nil
	}
	log.Printf("match.queue.join group found: mode=%s room_id=%s teams=%+v", mode, roomID, matchTeams)

	matchID, err := s.ms.CreateMatchFromQueue(ctx, &matchqueue.MatchQueueResult{
		RoomID: roomID,
		Mode:   mode,
		Teams:  matchTeams,
	})
	if err != nil {
		log.Printf("match.queue.join create match failed: mode=%s room_id=%s teams=%+v err=%v", mode, roomID, matchTeams, err)
		if err := s.restoreMatchedUsers(ctx, matchTeams, mode); err != nil {
			return nil, fmt.Errorf("恢复匹配用户失败: %w", err)
		}
		return nil, errors.New("创建比赛失败")
	}
	log.Printf("match.queue.join match created: match_id=%d room_id=%s mode=%s", matchID, roomID, mode)
	if err := s.updateUserStateToMatched(ctx, matchTeams, roomID, matchID); err != nil {
		return nil, fmt.Errorf("更新用户匹配状态失败: %w", err)
	}
	log.Printf("match.queue.join users matched: match_id=%d room_id=%s", matchID, roomID)
	chunkID := ""
	if s.mineMatchService != nil && mode == "1v1" {
		chunkID, err = s.mineMatchService.AssignChunkForMatch(ctx, matchID, roomID, matchTeams)
		if err != nil {
			log.Printf("match.queue.join assign mine chunk failed: match_id=%d room_id=%s err=%v", matchID, roomID, err)
			if restoreErr := s.restoreMatchedUsers(ctx, matchTeams, mode); restoreErr != nil {
				return nil, fmt.Errorf("分配扫雷 Chunk 失败: %w; 恢复匹配用户失败: %w", err, restoreErr)
			}
			return nil, fmt.Errorf("分配扫雷 Chunk 失败: %w", err)
		}
		if err := s.updateUserStateChunk(ctx, matchTeams, chunkID); err != nil {
			return nil, fmt.Errorf("更新扫雷 Chunk 状态失败: %w", err)
		}
		log.Printf("match.queue.join mine chunk assigned: match_id=%d room_id=%s chunk_id=%s", matchID, roomID, chunkID)
	}
	if _, err := s.rs.CreateRoom(roomID, mode, matchID, matchTeams); err != nil {
		log.Printf("match.queue.join create room failed: match_id=%d room_id=%s err=%v", matchID, roomID, err)
		return nil, errors.New("创建房间失败")
	}
	log.Printf("match.queue.join room created: match_id=%d room_id=%s mode=%s", matchID, roomID, mode)
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
		ChunkID:   status.ChunkID,
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
		ChunkID:   status.ChunkID,
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
		ChunkID:       state.ChunkID,
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
	log.Printf("len: %d", len)
	log.Printf("required: %d", required)
	if int(len) < int(required) {
		log.Printf("match.queue.find group not enough players: mode=%s required=%d current=%d", mode, required, len)
		return nil, "", nil
	}
	userIDs, err := s.repo.DequeueBatch(ctx, mode, int64(required))
	if err != nil {
		return nil, "", fmt.Errorf("从队列中批量弹出用户失败 for mode %s: %w", mode, err)
	}
	log.Printf("match.queue.find dequeued: mode=%s required=%d user_ids=%v", mode, required, userIDs)

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

func (s *MatchQueueService) updateUserStateChunk(ctx context.Context, teams []matchqueue.MatchedTeam, chunkID string) error {
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
			state.ChunkID = chunkID
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
			userState.Status = matchqueue.QueueStatusMatching
			userState.MatchID = 0
			userState.RoomID = ""
			userState.ChunkID = ""
			userState.UpdatedAt = time.Now()
			if err := s.repo.SetUserStatus(ctx, userState); err != nil {
				return fmt.Errorf("恢复用户匹配状态失败 for userID %d: %w", userID, err)
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
