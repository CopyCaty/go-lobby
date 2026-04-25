package service

import (
	"context"
	"fmt"
	"go-lobby/internal/matchqueue"
	"go-lobby/internal/model"
	"go-lobby/internal/repository"
	"time"
)

type MatchService struct {
	repo *repository.MatchRepository
}

func NewMatchService(repo *repository.MatchRepository) *MatchService {
	return &MatchService{repo: repo}
}

func (s *MatchService) CreateMatchFromQueue(ctx context.Context, matchResult *matchqueue.MatchQueueResult) (int64, error) {
	fmt.Println("enter match create")
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		fmt.Println("Begin Transaction Failed!")
		return 0, err
	}
	commit := false
	defer func() {
		if !commit {
			_ = tx.Rollback()
		}
	}()
	match := &model.Match{
		Status:    model.MatchStatusOngoing,
		StartedAt: time.Now(),
		Mode:      matchResult.Mode,
		RoomID:    matchResult.RoomID,
	}
	matchID, err := s.repo.CreateMatch(ctx, tx, match)
	if err != nil {
		fmt.Println("Create Match Failed!")
		return 0, err
	}
	matchPlayers := buildPlayerFromMatch(matchID, matchResult.Teams)
	err = s.repo.AddMatchPlayer(ctx, tx, matchPlayers)
	if err != nil {
		fmt.Println("Add Match PlayerFailed!")
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		fmt.Println("Commit Transaction Failed!")
		return 0, err
	}
	fmt.Printf("Match Created! MatchID: %d, RoomID: %s\n", matchID, matchResult.RoomID)
	commit = true
	return matchID, nil
}

func buildPlayerFromMatch(matchID int64, teams []matchqueue.MatchedTeam) []*model.MatchPlayer {
	players := make([]*model.MatchPlayer, 0)
	for _, team := range teams {
		for _, userID := range team.UserIDs {
			players = append(players, &model.MatchPlayer{
				MatchID: matchID,
				UserID:  userID,
				TeamNo:  team.TeamID,
			})
		}
	}
	return players
}
