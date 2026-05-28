package res

import "time"

type OpenMineCellResponse struct {
	ChunkID       string               `json:"chunk_id"`
	X             int                  `json:"x"`
	Y             int                  `json:"y"`
	Index         int                  `json:"index"`
	Mine          bool                 `json:"mine"`
	Closed        bool                 `json:"closed"`
	Canceled      bool                 `json:"canceled"`
	Reason        string               `json:"reason,omitempty"`
	AdjacentMines int                  `json:"adjacent_mines"`
	Version       int64                `json:"version"`
	OpenedAt      time.Time            `json:"opened_at"`
	OpenedCells   []OpenedCellResponse `json:"opened_cells,omitempty"`
}

type FlagMineCellResponse struct {
	ChunkID string `json:"chunk_id"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Index   int    `json:"index"`
	Flagged bool   `json:"flagged"`
	Version int64  `json:"version"`
}
