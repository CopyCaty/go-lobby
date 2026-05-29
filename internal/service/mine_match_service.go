package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"go-lobby/internal/dto/res"
	"go-lobby/internal/matchqueue"
	"go-lobby/internal/model"
)

var (
	ErrMineMatchChunkOccupied = errors.New("chunk is occupied by mine match")
	ErrMineMatchWrongChunk    = errors.New("mine match player can only operate assigned chunk")
	ErrMineMatchNoChunk       = errors.New("no clean chunk available")
)

type MineMatchStore interface {
	GetMatchState(ctx context.Context, matchID int64) (*model.MineMatchState, error)
	SaveMatchState(ctx context.Context, state *model.MineMatchState) error
	GetActiveMatchByUser(ctx context.Context, userID int64) (*model.MineMatchState, error)
	SetActiveMatchForUsers(ctx context.Context, state *model.MineMatchState) error
	DeleteActiveMatchForUsers(ctx context.Context, state *model.MineMatchState) error
	GetChunkLock(ctx context.Context, seasonID int64, chunkID string) (int64, error)
	TryLockChunk(ctx context.Context, seasonID int64, chunkID string, matchID int64) (bool, error)
	UnlockChunk(ctx context.Context, seasonID int64, chunkID string, matchID int64) error
}

type MineMatchService struct {
	seasonReader MineSeasonReader
	chunkStore   MineChunkStateStore
	matchStore   MineMatchStore
	matchService *MatchService
	mineGen      *model.MineGenerator
	rand         *rand.Rand
}

func NewMineMatchService(
	seasonReader MineSeasonReader,
	chunkStore MineChunkStateStore,
	matchStore MineMatchStore,
	matchService *MatchService,
) *MineMatchService {
	return &MineMatchService{
		seasonReader: seasonReader,
		chunkStore:   chunkStore,
		matchStore:   matchStore,
		matchService: matchService,
		mineGen:      model.NewMineGenerator(),
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *MineMatchService) AssignChunkForMatch(ctx context.Context, matchID int64, roomID string, teams []matchqueue.MatchedTeam) (string, error) {
	if s == nil || s.seasonReader == nil || s.chunkStore == nil || s.matchStore == nil {
		log.Printf("mine.match.assign skipped: missing deps match_id=%d room_id=%s", matchID, roomID)
		return "", nil
	}
	log.Printf("mine.match.assign start: match_id=%d room_id=%s teams=%+v", matchID, roomID, teams)
	season, err := s.seasonReader.GetActiveSeason(ctx)
	if err != nil {
		log.Printf("mine.match.assign active season failed: match_id=%d err=%v", matchID, err)
		return "", err
	}
	if season == nil {
		log.Printf("mine.match.assign no active season: match_id=%d", matchID)
		return "", ErrNoActiveSeason
	}
	log.Printf("mine.match.assign active season: match_id=%d season_id=%d season_code=%s", matchID, season.ID, season.Code)

	chunkID, err := s.findAndLockCleanChunk(ctx, season.ID, matchID)
	if err != nil {
		log.Printf("mine.match.assign find chunk failed: match_id=%d season_id=%d err=%v", matchID, season.ID, err)
		return "", err
	}
	state := buildMineMatchState(matchID, roomID, season.ID, chunkID, teams)
	if err := s.matchStore.SaveMatchState(ctx, state); err != nil {
		log.Printf("mine.match.assign save state failed: match_id=%d chunk_id=%s err=%v", matchID, chunkID, err)
		_ = s.matchStore.UnlockChunk(ctx, season.ID, chunkID, matchID)
		return "", err
	}
	if err := s.matchStore.SetActiveMatchForUsers(ctx, state); err != nil {
		log.Printf("mine.match.assign set users failed: match_id=%d chunk_id=%s err=%v", matchID, chunkID, err)
		_ = s.matchStore.UnlockChunk(ctx, season.ID, chunkID, matchID)
		return "", err
	}
	log.Printf("mine.match.assign success: match_id=%d room_id=%s season_id=%d chunk_id=%s", matchID, roomID, season.ID, chunkID)
	return chunkID, nil
}

func (s *MineMatchService) AuthorizeChunkOperation(ctx context.Context, userID int64, seasonID int64, chunkID string) (*model.MineMatchState, error) {
	if s == nil || s.matchStore == nil {
		return nil, nil
	}
	activeMatch, err := s.matchStore.GetActiveMatchByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if activeMatch != nil {
		if activeMatch.ChunkID != chunkID {
			return nil, ErrMineMatchWrongChunk
		}
		return activeMatch, nil
	}
	lockedMatchID, err := s.matchStore.GetChunkLock(ctx, seasonID, chunkID)
	if err != nil {
		return nil, err
	}
	if lockedMatchID != 0 {
		return nil, ErrMineMatchChunkOccupied
	}
	return nil, nil
}

func (s *MineMatchService) ApplyOpenResult(
	ctx context.Context,
	userID int64,
	season *model.MineSeason,
	chunkID model.ChunkID,
	state *model.MineChunkState,
	openResp *res.OpenMineCellResponse,
) error {
	if s == nil || s.matchStore == nil || season == nil || state == nil || openResp == nil {
		return nil
	}
	matchState, err := s.matchStore.GetActiveMatchByUser(ctx, userID)
	if err != nil {
		return err
	}
	if matchState == nil || matchState.ChunkID != chunkID.String() {
		return nil
	}
	attachMineMatchResponse(openResp, matchState)
	if openResp.Canceled {
		return nil
	}
	if openResp.Mine {
		winTeamNo, err := s.opponentTeam(matchState, userID)
		if err != nil {
			return err
		}
		return s.finishMatch(ctx, matchState, winTeamNo, time.Now(), openResp)
	}
	if openResp.NewOpenedCount > 0 {
		matchState.PlayerScores[userID] += openResp.NewOpenedCount
		matchState.LastScoreAt[userID] = time.Now().UnixNano()
		if err := s.matchStore.SaveMatchState(ctx, matchState); err != nil {
			return err
		}
		attachMineMatchResponse(openResp, matchState)
	}
	completed, err := s.allSafeCellsOpened(season, chunkID, state)
	if err != nil {
		return err
	}
	if !completed {
		return nil
	}
	winTeamNo, err := s.scoreWinnerTeam(matchState)
	if err != nil {
		return err
	}
	return s.finishMatch(ctx, matchState, winTeamNo, time.Now(), openResp)
}

func (s *MineMatchService) findAndLockCleanChunk(ctx context.Context, seasonID int64, matchID int64) (string, error) {
	gridSize, err := model.ChunkGridSize(model.ChunkMaxLevel)
	if err != nil {
		return "", err
	}
	total := gridSize * gridSize
	order := s.rand.Perm(total)
	for _, value := range order {
		chunkID := model.ChunkID{
			Region: "cn",
			Z:      model.ChunkMaxLevel,
			X:      value % gridSize,
			Y:      value / gridSize,
		}.String()
		clean, err := s.isCleanUnlockedChunk(ctx, seasonID, chunkID)
		if err != nil {
			return "", err
		}
		if !clean {
			continue
		}
		locked, err := s.matchStore.TryLockChunk(ctx, seasonID, chunkID, matchID)
		if err != nil {
			log.Printf("mine.match.find chunk lock failed: match_id=%d season_id=%d chunk_id=%s err=%v", matchID, seasonID, chunkID, err)
			return "", err
		}
		if !locked {
			log.Printf("mine.match.find chunk already locked: match_id=%d season_id=%d chunk_id=%s", matchID, seasonID, chunkID)
			continue
		}
		clean, err = s.isCleanChunk(ctx, seasonID, chunkID)
		if err != nil {
			_ = s.matchStore.UnlockChunk(ctx, seasonID, chunkID, matchID)
			return "", err
		}
		if !clean {
			_ = s.matchStore.UnlockChunk(ctx, seasonID, chunkID, matchID)
			continue
		}
		return chunkID, nil
	}
	return "", ErrMineMatchNoChunk
}

func (s *MineMatchService) isCleanUnlockedChunk(ctx context.Context, seasonID int64, chunkID string) (bool, error) {
	lockedMatchID, err := s.matchStore.GetChunkLock(ctx, seasonID, chunkID)
	if err != nil {
		return false, err
	}
	if lockedMatchID != 0 {
		return false, nil
	}
	return s.isCleanChunk(ctx, seasonID, chunkID)
}

func (s *MineMatchService) isCleanChunk(ctx context.Context, seasonID int64, chunkID string) (bool, error) {
	state, err := s.chunkStore.GetState(ctx, seasonID, chunkID)
	if err != nil {
		return false, err
	}
	if state == nil {
		return true, nil
	}
	return !state.Closed && len(state.OpenedCells) == 0 && len(state.FlaggedBy) == 0, nil
}

func (s *MineMatchService) allSafeCellsOpened(season *model.MineSeason, chunkID model.ChunkID, state *model.MineChunkState) (bool, error) {
	if len(state.OpenedCells) == 0 {
		return false, nil
	}
	safeCells := 0
	for y := 0; y < model.ChunkSize; y++ {
		for x := 0; x < model.ChunkSize; x++ {
			isMine, err := s.mineGen.IsMine(season, chunkID, x, y)
			if err != nil {
				return false, err
			}
			if !isMine {
				safeCells++
			}
		}
	}
	return len(state.OpenedCells) >= safeCells, nil
}

func (s *MineMatchService) opponentTeam(state *model.MineMatchState, userID int64) (int8, error) {
	userTeam, ok := state.UserTeams[userID]
	if !ok {
		return 0, fmt.Errorf("user %d is not in mine match %d", userID, state.MatchID)
	}
	for teamNo := range state.Teams {
		if teamNo != userTeam {
			return teamNo, nil
		}
	}
	return 0, fmt.Errorf("opponent team not found for mine match %d", state.MatchID)
}

func (s *MineMatchService) scoreWinnerTeam(state *model.MineMatchState) (int8, error) {
	var winnerTeam int8
	winnerSet := false
	winnerScore := -1
	var winnerLastScoreAt int64
	for teamNo, userIDs := range state.Teams {
		teamScore := 0
		var teamLastScoreAt int64
		for _, userID := range userIDs {
			teamScore += state.PlayerScores[userID]
			lastScoreAt := state.LastScoreAt[userID]
			if lastScoreAt > 0 && (teamLastScoreAt == 0 || lastScoreAt < teamLastScoreAt) {
				teamLastScoreAt = lastScoreAt
			}
		}
		if !winnerSet ||
			teamScore > winnerScore ||
			(teamScore == winnerScore && teamLastScoreAt > 0 && (winnerLastScoreAt == 0 || teamLastScoreAt < winnerLastScoreAt)) ||
			(teamScore == winnerScore && teamLastScoreAt == winnerLastScoreAt && teamNo < winnerTeam) {
			winnerSet = true
			winnerTeam = teamNo
			winnerScore = teamScore
			winnerLastScoreAt = teamLastScoreAt
		}
	}
	if !winnerSet {
		return 0, fmt.Errorf("winner team not found for mine match %d", state.MatchID)
	}
	return winnerTeam, nil
}

func (s *MineMatchService) finishMatch(ctx context.Context, state *model.MineMatchState, winTeamNo int8, finishedAt time.Time, openResp *res.OpenMineCellResponse) error {
	if state.Status == model.MineMatchStatusFinished {
		attachMineMatchResponse(openResp, state)
		return nil
	}
	if s.matchService != nil {
		if err := s.matchService.SetMatchResult(ctx, state.MatchID, winTeamNo); err != nil {
			return err
		}
	}
	state.Status = model.MineMatchStatusFinished
	state.WinTeamNo = &winTeamNo
	state.FinishedAt = &finishedAt
	if err := s.matchStore.SaveMatchState(ctx, state); err != nil {
		return err
	}
	if err := s.matchStore.DeleteActiveMatchForUsers(ctx, state); err != nil {
		return err
	}
	if err := s.matchStore.UnlockChunk(ctx, state.SeasonID, state.ChunkID, state.MatchID); err != nil {
		return err
	}
	attachMineMatchResponse(openResp, state)
	return nil
}

func buildMineMatchState(matchID int64, roomID string, seasonID int64, chunkID string, teams []matchqueue.MatchedTeam) *model.MineMatchState {
	now := time.Now()
	state := &model.MineMatchState{
		MatchID:      matchID,
		RoomID:       roomID,
		SeasonID:     seasonID,
		ChunkID:      chunkID,
		Status:       model.MineMatchStatusOngoing,
		Teams:        make(map[int8][]int64),
		UserTeams:    make(map[int64]int8),
		PlayerScores: make(map[int64]int),
		LastScoreAt:  make(map[int64]int64),
		StartedAt:    now,
	}
	for _, team := range teams {
		state.Teams[team.TeamID] = append([]int64(nil), team.UserIDs...)
		for _, userID := range team.UserIDs {
			state.UserTeams[userID] = team.TeamID
			state.PlayerScores[userID] = 0
		}
	}
	return state
}

func attachMineMatchResponse(openResp *res.OpenMineCellResponse, state *model.MineMatchState) {
	if openResp == nil || state == nil {
		return
	}
	openResp.MatchID = state.MatchID
	openResp.MatchScores = make(map[int64]int, len(state.PlayerScores))
	for userID, score := range state.PlayerScores {
		openResp.MatchScores[userID] = score
	}
	openResp.MatchFinished = state.Status == model.MineMatchStatusFinished
	openResp.WinTeamNo = state.WinTeamNo
}
