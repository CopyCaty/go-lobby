package model

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	ChunkSize       = 128
	ChunkCellCount  = ChunkSize * ChunkSize
	ChunkBitmapSize = ChunkCellCount / 8
	ChunkMinLevel   = 0
	ChunkMaxLevel   = 6
)

type ChunkID struct {
	Region string
	Z      int
	X      int
	Y      int
}

func ParseChunkID(raw string) (ChunkID, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 4 {
		return ChunkID{}, fmt.Errorf("invalid chunk ID format")
	}
	if parts[0] == "" {
		return ChunkID{}, fmt.Errorf("chunk region cannot be empty")
	}

	z, err := parseChunkCoord("z", parts[1])
	if err != nil {
		return ChunkID{}, err
	}
	x, err := parseChunkCoord("x", parts[2])
	if err != nil {
		return ChunkID{}, err
	}
	y, err := parseChunkCoord("y", parts[3])
	if err != nil {
		return ChunkID{}, err
	}

	return ChunkID{
		Region: parts[0],
		Z:      z,
		X:      x,
		Y:      y,
	}, nil
}

func (id ChunkID) String() string {
	return fmt.Sprintf("%s:%d:%d:%d", id.Region, id.Z, id.X, id.Y)
}

type ChunkBounds struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
}

func ChunkGridSize(level int) (int, error) {
	if level < ChunkMinLevel || level > ChunkMaxLevel {
		return 0, fmt.Errorf("chunk level out of range")
	}
	return 1 << level, nil
}

func ChunkBoundsFor(level, x, y int) (ChunkBounds, error) {
	gridSize, err := ChunkGridSize(level)
	if err != nil {
		return ChunkBounds{}, err
	}
	if x < 0 || x >= gridSize || y < 0 || y >= gridSize {
		return ChunkBounds{}, fmt.Errorf("chunk coordinate out of range")
	}

	cellSize := 1 / float64(gridSize)
	return ChunkBounds{
		MinX: float64(x) * cellSize,
		MinY: float64(y) * cellSize,
		MaxX: float64(x+1) * cellSize,
		MaxY: float64(y+1) * cellSize,
	}, nil
}

func VisibleChunkRange(level int, bounds ChunkBounds) (minX, minY, maxX, maxY int, err error) {
	gridSize, err := ChunkGridSize(level)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if bounds.MinX < 0 || bounds.MinY < 0 || bounds.MaxX > 1 || bounds.MaxY > 1 ||
		bounds.MinX >= bounds.MaxX || bounds.MinY >= bounds.MaxY {
		return 0, 0, 0, 0, fmt.Errorf("invalid chunk bounds")
	}

	const epsilon = 0.000000001
	minX = clampChunkCoord(int(math.Floor(bounds.MinX*float64(gridSize))), gridSize)
	minY = clampChunkCoord(int(math.Floor(bounds.MinY*float64(gridSize))), gridSize)
	maxX = clampChunkCoord(int(math.Floor((bounds.MaxX-epsilon)*float64(gridSize))), gridSize)
	maxY = clampChunkCoord(int(math.Floor((bounds.MaxY-epsilon)*float64(gridSize))), gridSize)
	return minX, minY, maxX, maxY, nil
}

func ChunkCellIndex(x, y int) (int, error) {
	if x < 0 || x >= ChunkSize || y < 0 || y >= ChunkSize {
		return 0, fmt.Errorf("cell coordinate out of range")
	}
	return y*ChunkSize + x, nil
}

func parseChunkCoord(name string, raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid chunk %s", name)
	}
	if value < 0 {
		return 0, fmt.Errorf("chunk %s cannot be negative", name)
	}
	return value, nil
}

func clampChunkCoord(value, gridSize int) int {
	if value < 0 {
		return 0
	}
	if value >= gridSize {
		return gridSize - 1
	}
	return value
}

type Chunk struct {
	ID      string
	Width   int
	Height  int
	Closed  bool
	Mines   []byte
	Opened  []byte
	Flags   []byte
	Version int64
}
