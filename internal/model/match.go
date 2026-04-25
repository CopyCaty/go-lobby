package model

import "time"

const (
	MatchStatusOngoing  = 1
	MatchStatusFinished = 2
)

type Match struct {
	ID         int64     `db:"id"`
	Status     int8      `db:"status"`
	WinTeamNo  int8      `db:"win_team_no,omitempty"`
	StartedAt  time.Time `db:"started_at"`
	RoomID     string    `db:"room_id,omitempty"`
	FinishedAt time.Time `db:"finished_at,omitempty"`
	Mode       string    `db:"mode"`
}
