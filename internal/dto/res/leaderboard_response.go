package res

type LeaderboardResponse struct {
	Mode  string                    `json:"mode"`
	Items []LeaderboardItemResponse `json:"items"`
}

type LeaderboardItemResponse struct {
	Rank   int   `json:"rank"`
	UserID int64 `json:"user_id"`
	Score  int   `json:"score"`
}
