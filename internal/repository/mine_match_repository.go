package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go-lobby/internal/model"

	"github.com/redis/go-redis/v9"
)

type MineMatchRepository struct {
	rdb *redis.Client
}

func NewMineMatchRepository(rdb *redis.Client) *MineMatchRepository {
	return &MineMatchRepository{rdb: rdb}
}

func mineMatchKey(matchID int64) string {
	return fmt.Sprintf("go_lobby:mine_match:%d", matchID)
}

func mineMatchUserKey(userID int64) string {
	return fmt.Sprintf("go_lobby:mine_match_user:%d", userID)
}

func mineChunkLockKey(seasonID int64, chunkID string) string {
	return fmt.Sprintf("go_lobby:mine_chunk_lock:%d:%s", seasonID, chunkID)
}

func (r *MineMatchRepository) GetMatchState(ctx context.Context, matchID int64) (*model.MineMatchState, error) {
	data, err := r.rdb.Get(ctx, mineMatchKey(matchID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var state model.MineMatchState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	normalizeMineMatchState(&state)
	return &state, nil
}

func (r *MineMatchRepository) SaveMatchState(ctx context.Context, state *model.MineMatchState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, mineMatchKey(state.MatchID), data, 0).Err()
}

func (r *MineMatchRepository) GetActiveMatchByUser(ctx context.Context, userID int64) (*model.MineMatchState, error) {
	rawMatchID, err := r.rdb.Get(ctx, mineMatchUserKey(userID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	matchID, err := strconv.ParseInt(rawMatchID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid mine match id for user %d: %w", userID, err)
	}
	state, err := r.GetMatchState(ctx, matchID)
	if err != nil || state == nil {
		return state, err
	}
	if state.Status != model.MineMatchStatusOngoing {
		return nil, nil
	}
	return state, nil
}

func (r *MineMatchRepository) SetActiveMatchForUsers(ctx context.Context, state *model.MineMatchState) error {
	pipe := r.rdb.TxPipeline()
	for userID := range state.UserTeams {
		pipe.Set(ctx, mineMatchUserKey(userID), state.MatchID, 0)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *MineMatchRepository) DeleteActiveMatchForUsers(ctx context.Context, state *model.MineMatchState) error {
	pipe := r.rdb.TxPipeline()
	for userID := range state.UserTeams {
		pipe.Del(ctx, mineMatchUserKey(userID))
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *MineMatchRepository) GetChunkLock(ctx context.Context, seasonID int64, chunkID string) (int64, error) {
	rawMatchID, err := r.rdb.Get(ctx, mineChunkLockKey(seasonID, chunkID)).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}
	matchID, err := strconv.ParseInt(rawMatchID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid mine chunk lock for %s: %w", chunkID, err)
	}
	return matchID, nil
}

func (r *MineMatchRepository) TryLockChunk(ctx context.Context, seasonID int64, chunkID string, matchID int64) (bool, error) {
	return r.rdb.SetNX(ctx, mineChunkLockKey(seasonID, chunkID), matchID, 0).Result()
}

func (r *MineMatchRepository) UnlockChunk(ctx context.Context, seasonID int64, chunkID string, matchID int64) error {
	key := mineChunkLockKey(seasonID, chunkID)
	rawMatchID, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}
	lockedMatchID, err := strconv.ParseInt(rawMatchID, 10, 64)
	if err != nil {
		return err
	}
	if lockedMatchID != matchID {
		return nil
	}
	return r.rdb.Del(ctx, key).Err()
}

func normalizeMineMatchState(state *model.MineMatchState) {
	if state.Teams == nil {
		state.Teams = make(map[int8][]int64)
	}
	if state.UserTeams == nil {
		state.UserTeams = make(map[int64]int8)
	}
	if state.PlayerScores == nil {
		state.PlayerScores = make(map[int64]int)
	}
	if state.LastScoreAt == nil {
		state.LastScoreAt = make(map[int64]int64)
	}
}
