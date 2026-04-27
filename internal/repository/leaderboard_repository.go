package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type LeaderboardRepository struct {
	rdb *redis.Client
}

func NewLeaderboardRepository(rdb *redis.Client) *LeaderboardRepository {
	return &LeaderboardRepository{
		rdb: rdb,
	}
}

func (r *LeaderboardRepository) SetPlayerScore(ctx context.Context, mode string, userID int64, score int) error {
	return r.rdb.ZAdd(ctx, leaderboardKey(mode), redis.Z{
		Score:  float64(score),
		Member: strconv.FormatInt(userID, 10),
	}).Err()
}

func (r *LeaderboardRepository) ListTopN(ctx context.Context, mode string, limit int64) ([]LeaderboardRank, error) {
	values, err := r.rdb.ZRevRangeWithScores(ctx, leaderboardKey(mode), 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	ranks := make([]LeaderboardRank, 0, len(values))
	for _, value := range values {
		userID, err := strconv.ParseInt(fmt.Sprint(value.Member), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("排行榜用户ID错误 :%v", value.Member)
		}
		ranks = append(ranks, LeaderboardRank{
			UserID: userID,
			Score:  int(value.Score),
		})
	}
	return ranks, nil
}

type LeaderboardRank struct {
	UserID int64
	Score  int
}

func leaderboardKey(mode string) string {
	return fmt.Sprintf("go_lobby:leaderboard:mode:%s", mode)
}
