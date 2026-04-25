package res

type MatchPlayerResponse struct {
	MatchID int64 `json:"match_id"`
	UserID  int64 `json:"user_id"`
	TeamNo  int8  `json:"team_no"`
}
