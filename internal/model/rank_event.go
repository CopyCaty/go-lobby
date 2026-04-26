package model

import "time"

type RankEvent struct {
	ID        int64     `db:"id"`
	EventID   string    `db:"event_id"`
	MatchID   int64     `db:"match_id"`
	Status    int8      `db:"status"`
	CreatedAt time.Time `db:"created_at"`
}
