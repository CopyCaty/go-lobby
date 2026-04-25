package model

import "time"

type MatchPlayer struct {
	ID        int64     `db:"id"`
	MatchID   int64     `db:"match_id"`
	UserID    int64     `db:"user_id"`
	TeamNo    int8      `db:"team_no"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
