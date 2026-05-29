package model

import "time"

const (
	MineMatchStatusOngoing  = "ongoing"
	MineMatchStatusFinished = "finished"
)

type MineMatchState struct {
	MatchID      int64            `json:"match_id"`
	RoomID       string           `json:"room_id"`
	SeasonID     int64            `json:"season_id"`
	ChunkID      string           `json:"chunk_id"`
	Status       string           `json:"status"`
	Teams        map[int8][]int64 `json:"teams"`
	UserTeams    map[int64]int8   `json:"user_teams"`
	PlayerScores map[int64]int    `json:"player_scores"`
	LastScoreAt  map[int64]int64  `json:"last_score_at"`
	WinTeamNo    *int8            `json:"win_team_no,omitempty"`
	StartedAt    time.Time        `json:"started_at"`
	FinishedAt   *time.Time       `json:"finished_at,omitempty"`
}
