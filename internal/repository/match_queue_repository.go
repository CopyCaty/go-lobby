package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"go-lobby/internal/matchqueue"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type MatchQueueRepository struct {
	rdb *redis.Client
}

func NewMatchQueueRepository(rdb *redis.Client) *MatchQueueRepository {
	return &MatchQueueRepository{
		rdb: rdb,
	}
}

func queueUserKey(userID int64) string {
	return fmt.Sprintf("go_lobby:queue_user:%d", userID)
}

func queueKey(mode string) string {
	return fmt.Sprintf("go_lobby:queue:%s", mode)
}

func (r *MatchQueueRepository) Enqueue(ctx context.Context, mode string, userID int64) error {
	return r.rdb.RPush(ctx, queueKey(mode), userID).Err()
}

func (r *MatchQueueRepository) QueueLen(ctx context.Context, mode string) (int64, error) {
	return r.rdb.LLen(ctx, queueKey(mode)).Result()
}

func (r *MatchQueueRepository) DequeueBatch(ctx context.Context, mode string, count int64) ([]int64, error) {
	values, err := r.rdb.LPopCount(ctx, queueKey(mode), int(count)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	userIDs := make([]int64, len(values))
	for i, v := range values {
		userID, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid userID in queue: %s", v)
		}
		userIDs[i] = userID
	}
	return userIDs, nil

}

func (r *MatchQueueRepository) RemoveFromQueue(ctx context.Context, mode string, userID int64) error {
	return r.rdb.LRem(ctx, queueKey(mode), 0, userID).Err()
}

func (r *MatchQueueRepository) GetUserStatus(ctx context.Context, userID int64) (*matchqueue.QueueUserState, error) {
	data, err := r.rdb.Get(ctx, queueUserKey(userID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var state matchqueue.QueueUserState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *MatchQueueRepository) SetUserStatus(ctx context.Context, state *matchqueue.QueueUserState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, queueUserKey(state.UserID), data, 0).Err()
}

func (r *MatchQueueRepository) DeleteUserStatus(ctx context.Context, userID int64) error {
	return r.rdb.Del(ctx, queueUserKey(userID)).Err()
}
