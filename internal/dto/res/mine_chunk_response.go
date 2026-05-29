package res

import "time"

type OpenMineCellResponse struct {
	ChunkID        string               `json:"chunk_id"`
	X              int                  `json:"x"`
	Y              int                  `json:"y"`
	Index          int                  `json:"index"`
	Mine           bool                 `json:"mine"`
	Closed         bool                 `json:"closed"`
	ClosedAt       *time.Time           `json:"closed_at,omitempty"`
	ClosedUntil    *time.Time           `json:"closed_until,omitempty"`
	Canceled       bool                 `json:"canceled"`
	Reason         string               `json:"reason,omitempty"`
	AdjacentMines  int                  `json:"adjacent_mines"`
	Version        int64                `json:"version"`
	OpenedAt       time.Time            `json:"opened_at"`
	NewOpenedCount int                  `json:"new_opened_count"`
	MatchID        int64                `json:"match_id,omitempty"`
	MatchScores    map[int64]int        `json:"match_scores,omitempty"`
	MatchFinished  bool                 `json:"match_finished"`
	WinTeamNo      *int8                `json:"win_team_no,omitempty"`
	OpenedCells    []OpenedCellResponse `json:"opened_cells,omitempty"`
}

type FlagMineCellResponse struct {
	ChunkID string `json:"chunk_id"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Index   int    `json:"index"`
	Flagged bool   `json:"flagged"`
	Version int64  `json:"version"`
}
