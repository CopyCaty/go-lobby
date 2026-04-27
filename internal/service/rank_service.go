package service

import (
	"context"
	"fmt"
	"go-lobby/internal/dto/res"
	"go-lobby/internal/event"
	"go-lobby/internal/model"
	"go-lobby/internal/repository"
	"sync"
)

type RankService struct {
	mu              sync.Mutex
	rankRepo        *repository.RankRepository
	matchRepo       *repository.MatchRepository
	leaderboardRepo *repository.LeaderboardRepository
}

func NewRankService(
	rankRepo *repository.RankRepository,
	matchRepo *repository.MatchRepository,
	leaderboardRepo *repository.LeaderboardRepository,
) *RankService {
	return &RankService{
		rankRepo:        rankRepo,
		matchRepo:       matchRepo,
		leaderboardRepo: leaderboardRepo,
	}
}

func (s *RankService) SettleMatchResult(ctx context.Context, evt *event.MatchResultFinishedEvent) error {
	matchPlayers, err := s.matchRepo.GetMatchPlayers(ctx, evt.MatchID)
	if err != nil {
		return fmt.Errorf("查询比赛玩家失败: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.rankRepo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("开启排行榜事务失败: %w", err)
	}
	_, err = s.rankRepo.AddRankEvent(ctx, tx, &model.RankEvent{
		EventID: evt.EventID,
		MatchID: evt.MatchID,
		Status:  1,
	})
	if repository.IsDuplicateKeyError(err) {
		_ = tx.Rollback()
		return nil
	}
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("更新排名事件失败: %w", err)
	}
	for _, matchPlayer := range matchPlayers {
		var winCount, scoreDelta, loseCount int
		if matchPlayer.TeamNo == evt.WinTeamNo {
			scoreDelta = 100
			winCount = 1
			loseCount = 0
		} else {
			scoreDelta = -60
			loseCount = 1
			winCount = 0
		}
		_, err := s.rankRepo.UpdatePlayerRank(
			ctx,
			tx,
			evt.Mode,
			matchPlayer.UserID,
			scoreDelta,
			winCount,
			loseCount,
		)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("更新玩家积分失败")
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交排行榜事务失败: %w", err)
	}
	for _, matchPlayer := range matchPlayers {
		playerRank, err := s.rankRepo.GetPlayerRank(ctx, evt.Mode, matchPlayer.UserID)
		if err != nil {
			return fmt.Errorf("查询玩家积分失败: %w", err)
		}
		if playerRank == nil {
			continue
		}
		if err := s.leaderboardRepo.SetPlayerScore(ctx, evt.Mode, matchPlayer.UserID, playerRank.Score); err != nil {
			return fmt.Errorf("同步Redis排行榜失败: %w", err)
		}
	}
	return nil
}

func (s *RankService) GetLeaderboard(ctx context.Context, mode string, limit int64) (*res.LeaderboardResponse, error) {
	if mode == "" {
		return nil, fmt.Errorf("mode 不能为空")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	items, err := s.leaderboardRepo.ListTopN(ctx, mode, limit)
	if err != nil {
		return nil, fmt.Errorf("查询排行榜失败: %w", err)
	}
	respItems := make([]res.LeaderboardItemResponse, 0, len(items))
	for i, item := range items {
		respItems = append(respItems, res.LeaderboardItemResponse{
			Rank:   i + 1,
			UserID: item.UserID,
			Score:  item.Score,
		})
	}
	return &res.LeaderboardResponse{
		Mode:  mode,
		Items: respItems,
	}, nil
}
