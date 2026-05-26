package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"go-lobby/internal/model"

	"github.com/redis/go-redis/v9"
)

type MineChunkStateRepository struct {
	rdb *redis.Client
}

func NewMineChunkStateRepository(rdb *redis.Client) *MineChunkStateRepository {
	return &MineChunkStateRepository{rdb: rdb}
}

func mineChunkStateKey(seasonID int64, chunkID string) string {
	return fmt.Sprintf("go_lobby:mine:season:%d:chunk:%s", seasonID, chunkID)
}

func (r *MineChunkStateRepository) GetState(ctx context.Context, seasonID int64, chunkID string) (*model.MineChunkState, error) {
	data, err := r.rdb.Get(ctx, mineChunkStateKey(seasonID, chunkID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var state model.MineChunkState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.OpenedCells == nil {
		state.OpenedCells = make(map[int]model.MineOpenedCellSnapshot)
	}
	if state.FlaggedBy == nil {
		state.FlaggedBy = make(map[int]int64)
	}
	return &state, nil
}

func (r *MineChunkStateRepository) SaveState(ctx context.Context, state *model.MineChunkState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, mineChunkStateKey(state.SeasonID, state.ChunkID), data, 0).Err()
}
