package model

import (
	"errors"
	"fmt"
	"time"
)

var ErrChunkCellOutOfWorld = errors.New("chunk cell out of world")

type MineChunkState struct {
	SeasonID    int64                          `json:"season_id"`
	ChunkID     string                         `json:"chunk_id"`
	Closed      bool                           `json:"closed"`
	ClosedBy    int64                          `json:"closed_by,omitempty"`
	ClosedAt    *time.Time                     `json:"closed_at,omitempty"`
	Version     int64                          `json:"version"`
	OpenedCells map[int]MineOpenedCellSnapshot `json:"opened_cells"`
	FlaggedBy   map[int]int64                  `json:"flagged_by"`
}

type MineOpenedCellSnapshot struct {
	X             int       `json:"x"`
	Y             int       `json:"y"`
	Index         int       `json:"index"`
	OpenedBy      int64     `json:"opened_by"`
	OpenedAt      time.Time `json:"opened_at"`
	AdjacentMines int       `json:"adjacent_mines"`
}

func NewMineChunkState(seasonID int64, chunkID string) *MineChunkState {
	return &MineChunkState{
		SeasonID:    seasonID,
		ChunkID:     chunkID,
		Version:     1,
		OpenedCells: make(map[int]MineOpenedCellSnapshot),
		FlaggedBy:   make(map[int]int64),
	}
}

func NormalizeChunkCell(chunkID ChunkID, cellX, cellY int) (ChunkID, int, int, error) {
	gridSize, err := ChunkGridSize(chunkID.Z)
	if err != nil {
		return ChunkID{}, 0, 0, err
	}
	totalCells := gridSize * ChunkSize
	globalX := chunkID.X*ChunkSize + cellX
	globalY := chunkID.Y*ChunkSize + cellY
	if globalX < 0 || globalX >= totalCells || globalY < 0 || globalY >= totalCells {
		return ChunkID{}, 0, 0, ErrChunkCellOutOfWorld
	}

	normalized := ChunkID{
		Region: chunkID.Region,
		Z:      chunkID.Z,
		X:      globalX / ChunkSize,
		Y:      globalY / ChunkSize,
	}
	return normalized, globalX % ChunkSize, globalY % ChunkSize, nil
}

func EnsurePlayableChunk(chunkID ChunkID) error {
	if chunkID.Region != "cn" {
		return fmt.Errorf("unsupported region")
	}
	if chunkID.Z != ChunkMaxLevel {
		return fmt.Errorf("mine gameplay only supports max-level chunks")
	}
	gridSize, err := ChunkGridSize(chunkID.Z)
	if err != nil {
		return err
	}
	if chunkID.X < 0 || chunkID.X >= gridSize || chunkID.Y < 0 || chunkID.Y >= gridSize {
		return fmt.Errorf("chunk coordinate out of range")
	}
	return nil
}
