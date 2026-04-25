package res

import (
	"time"
)

type MatchInfoResponse struct {
	ID         int64                 `json:"id"`
	Status     int8                  `json:"status"`
	WinTeamNo  *int8                 `json:"win_team_no,omitempty"`
	StartedAt  time.Time             `json:"started_at"`
	RoomID     string                `json:"room_id,omitempty"`
	FinishedAt *time.Time            `json:"finished_at,omitempty"`
	Mode       string                `json:"mode"`
	Players    []MatchPlayerResponse `json:"players,omitempty"`
}
