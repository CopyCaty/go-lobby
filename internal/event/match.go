package event

import "time"

const MatchResultFinishedRoutingKey = "match.result.finished"

type MatchResultFinishedEvent struct {
	EventID    string    `json:"event_id"`
	MatchID    int64     `json:"match_id"`
	WinTeamNo  int8      `json:"win_team_no"`
	Mode       string    `json:"mode"`
	OccurredAt time.Time `json:"occurred_at"`
}
