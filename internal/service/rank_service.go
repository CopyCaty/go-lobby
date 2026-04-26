package service

import (
	"context"
	"fmt"
	"go-lobby/internal/event"
	"go-lobby/internal/model"
	"go-lobby/internal/repository"
	"sync"
)

type RankService struct {
	mu        sync.Mutex
	rankRepo  *repository.RankRepository
	matchRepo *repository.MatchRepository
}

func NewRankService(rankRepo *repository.RankRepository, matchRepo *repository.MatchRepository) *RankService {
	return &RankService{
		rankRepo:  rankRepo,
		matchRepo: matchRepo,
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
			tx.Rollback()
			return fmt.Errorf("更新玩家积分失败")
		}
	}
	tx.Commit()
	return nil
}
