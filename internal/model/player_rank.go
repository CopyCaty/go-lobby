package model

import "time"

type PlayerRank struct {
	ID         int64     `db:"id"`
	Mode       string    `db:"mode"`
	UserID     int64     `db:"user_id"`
	Score      int       `db:"score"`
	WinCount   int       `db:"win_count"`
	LoseCount  int       `db:"lose_count"`
	MatchCount int       `db:"match_count"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}
