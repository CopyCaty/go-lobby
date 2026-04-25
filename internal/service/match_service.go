package service

import (
	"context"
	"fmt"
	"go-lobby/internal/dto/res"
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

func (s *MatchService) SetMatchResult(ctx context.Context, matchID int64, winTeamNo int8) error {
	match, err := s.repo.GetMatchByID(ctx, matchID)
	if err != nil {
		return err
	}
	if match == nil {
		return fmt.Errorf("match with ID %d not found", matchID)
	}
	match.WinTeamNo = &winTeamNo
	match.Status = model.MatchStatusFinished
	now := time.Now()
	match.FinishedAt = &now
	_, err = s.repo.UpdateMatch(ctx, match)
	return err
}

func (s *MatchService) GetMatchInfo(ctx context.Context, matchID int64) (*res.MatchInfoResponse, error) {
	match, err := s.repo.GetMatchByID(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, fmt.Errorf("match with ID %d not found", matchID)
	}

	matchPlayers := s.getPlayerFromMatch(ctx, matchID)
	matchPlayerResponses := make([]res.MatchPlayerResponse, 0, len(matchPlayers))
	for _, mp := range matchPlayers {
		matchPlayerResponses = append(matchPlayerResponses, res.MatchPlayerResponse{
			MatchID: mp.MatchID,
			UserID:  mp.UserID,
			TeamNo:  mp.TeamNo,
		})
	}

	matchInfo := &res.MatchInfoResponse{
		ID:         match.ID,
		Status:     match.Status,
		WinTeamNo:  match.WinTeamNo,
		StartedAt:  match.StartedAt,
		RoomID:     match.RoomID,
		FinishedAt: match.FinishedAt,
		Mode:       match.Mode,
		Players:    matchPlayerResponses,
	}

	return matchInfo, nil
}

func (s *MatchService) getPlayerFromMatch(ctx context.Context, matchID int64) []*model.MatchPlayer {
	players, err := s.repo.GetMatchPlayers(ctx, matchID)
	if err != nil {
		fmt.Printf("Error occurred while fetching match players for match ID %d: %v\n", matchID, err)
		return nil
	}
	return players
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
