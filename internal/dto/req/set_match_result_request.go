package req

type SetMatchResultRequest struct {
	MatchID   int64 `json:"match_id"`
	WinTeamNo int8  `json:"win_team_no"`
}
