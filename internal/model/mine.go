package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

const (
	MineSeasonStatusActive = 1
	MineAlgorithmHMACV1    = 1
	DefaultMineRate        = 0.15
)

type MineSeason struct {
	ID               int64      `db:"id"`
	Code             string     `db:"code"`
	Seed             string     `db:"seed"`
	MineRate         float64    `db:"mine_rate"`
	AlgorithmVersion int        `db:"algorithm_version"`
	Status           int        `db:"status"`
	StartedAt        time.Time  `db:"started_at"`
	EndedAt          *time.Time `db:"ended_at"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

type MineGenerator struct{}

func NewMineGenerator() *MineGenerator {
	return &MineGenerator{}
}

func (g *MineGenerator) IsMine(season *MineSeason, chunkID ChunkID, cellX, cellY int) (bool, error) {
	if season == nil {
		return false, fmt.Errorf("mine season is nil")
	}
	normalized, x, y, err := NormalizeChunkCell(chunkID, cellX, cellY)
	if err != nil {
		return false, err
	}
	rate := season.MineRate
	if rate <= 0 || rate >= 1 {
		return false, fmt.Errorf("invalid mine rate")
	}

	globalX := normalized.X*ChunkSize + x
	globalY := normalized.Y*ChunkSize + y
	msg := fmt.Sprintf("%s|%d|%s|%d|%d|%d", season.Code, season.AlgorithmVersion, normalized.Region, normalized.Z, globalX, globalY)
	mac := hmac.New(sha256.New, []byte(season.Seed))
	_, _ = mac.Write([]byte(msg))
	sum := mac.Sum(nil)
	value := binary.BigEndian.Uint64(sum[:8])
	threshold := uint64(rate * float64(math.MaxUint64))
	return value < threshold, nil
}

func (g *MineGenerator) AdjacentMineCount(season *MineSeason, chunkID ChunkID, cellX, cellY int) (int, error) {
	count := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			isMine, err := g.IsMine(season, chunkID, cellX+dx, cellY+dy)
			if err != nil {
				if err == ErrChunkCellOutOfWorld {
					continue
				}
				return 0, err
			}
			if isMine {
				count++
			}
		}
	}
	return count, nil
}
