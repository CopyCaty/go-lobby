package res

import (
	"go-lobby/internal/model"
	"time"
)

type ChunkSummaryResponse struct {
	ChunkID         string            `json:"chunk_id"`
	Region          string            `json:"region"`
	Z               int               `json:"z"`
	Level           int               `json:"level"`
	X               int               `json:"x"`
	Y               int               `json:"y"`
	Bounds          model.ChunkBounds `json:"bounds"`
	Width           int               `json:"width"`
	Height          int               `json:"height"`
	State           string            `json:"state"`
	Closed          bool              `json:"closed"`
	Version         int64             `json:"version"`
	OpenedCount     int               `json:"opened_count"`
	ClosedLeafCount int               `json:"closed_leaf_count"`
	TotalLeafCount  int               `json:"total_leaf_count"`
	ClosedRatio     float64           `json:"closed_ratio"`
}

type ChunkListResponse struct {
	Region string                 `json:"region"`
	Level  int                    `json:"level"`
	BBox   model.ChunkBounds      `json:"bbox"`
	Chunks []ChunkSummaryResponse `json:"chunks"`
}

type ChunkSnapshotResponse struct {
	ChunkID     string               `json:"chunk_id"`
	Width       int                  `json:"width"`
	Height      int                  `json:"height"`
	Closed      bool                 `json:"closed"`
	Version     int64                `json:"version"`
	OpenedCells []OpenedCellResponse `json:"opened_cells"`
}

type OpenedCellResponse struct {
	X             int                  `json:"x"`
	Y             int                  `json:"y"`
	Index         int                  `json:"index"`
	AdjacentMines int                  `json:"adjacent_mines"`
	OpenedBy      CellOpenedByResponse `json:"opened_by"`
	OpenedAt      time.Time            `json:"opened_at"`
}

type CellOpenedByResponse struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
}
