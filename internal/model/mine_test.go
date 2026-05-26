package model

import "testing"

func TestMineGeneratorIsDeterministic(t *testing.T) {
	season := testMineSeason("s1", "seed-a")
	chunkID := ChunkID{Region: "cn", Z: ChunkMaxLevel, X: 10, Y: 20}
	gen := NewMineGenerator()

	first, err := gen.IsMine(season, chunkID, 12, 34)
	if err != nil {
		t.Fatalf("IsMine returned error: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := gen.IsMine(season, chunkID, 12, 34)
		if err != nil {
			t.Fatalf("IsMine returned error: %v", err)
		}
		if got != first {
			t.Fatalf("mine result should be deterministic")
		}
	}
}

func TestMineGeneratorDifferentSeedChangesLayout(t *testing.T) {
	chunkID := ChunkID{Region: "cn", Z: ChunkMaxLevel, X: 10, Y: 20}
	gen := NewMineGenerator()
	seasonA := testMineSeason("s1", "seed-a")
	seasonB := testMineSeason("s2", "seed-b")

	different := false
	for y := 0; y < ChunkSize && !different; y++ {
		for x := 0; x < ChunkSize; x++ {
			a, err := gen.IsMine(seasonA, chunkID, x, y)
			if err != nil {
				t.Fatalf("IsMine returned error: %v", err)
			}
			b, err := gen.IsMine(seasonB, chunkID, x, y)
			if err != nil {
				t.Fatalf("IsMine returned error: %v", err)
			}
			if a != b {
				different = true
				break
			}
		}
	}
	if !different {
		t.Fatal("different seeds should produce a different layout in a chunk")
	}
}

func TestMineGeneratorMineRateIsClose(t *testing.T) {
	season := testMineSeason("s1", "seed-a")
	chunkID := ChunkID{Region: "cn", Z: ChunkMaxLevel, X: 10, Y: 20}
	gen := NewMineGenerator()

	mines := 0
	for y := 0; y < ChunkSize; y++ {
		for x := 0; x < ChunkSize; x++ {
			isMine, err := gen.IsMine(season, chunkID, x, y)
			if err != nil {
				t.Fatalf("IsMine returned error: %v", err)
			}
			if isMine {
				mines++
			}
		}
	}
	rate := float64(mines) / float64(ChunkCellCount)
	if rate < 0.13 || rate > 0.17 {
		t.Fatalf("mine rate too far from expected: %.4f", rate)
	}
}

func TestMineGeneratorAdjacentCountCrossesChunkBoundary(t *testing.T) {
	season := testMineSeason("s1", "seed-a")
	chunkID := ChunkID{Region: "cn", Z: ChunkMaxLevel, X: 10, Y: 20}
	gen := NewMineGenerator()

	got, err := gen.AdjacentMineCount(season, chunkID, ChunkSize-1, ChunkSize-1)
	if err != nil {
		t.Fatalf("AdjacentMineCount returned error: %v", err)
	}
	want := 0
	for _, cell := range []struct {
		x int
		y int
	}{
		{ChunkSize - 2, ChunkSize - 2},
		{ChunkSize - 1, ChunkSize - 2},
		{ChunkSize, ChunkSize - 2},
		{ChunkSize - 2, ChunkSize - 1},
		{ChunkSize, ChunkSize - 1},
		{ChunkSize - 2, ChunkSize},
		{ChunkSize - 1, ChunkSize},
		{ChunkSize, ChunkSize},
	} {
		isMine, err := gen.IsMine(season, chunkID, cell.x, cell.y)
		if err != nil {
			t.Fatalf("IsMine returned error: %v", err)
		}
		if isMine {
			want++
		}
	}
	if got != want {
		t.Fatalf("unexpected adjacent mine count: got %d want %d", got, want)
	}
}

func testMineSeason(code, seed string) *MineSeason {
	return &MineSeason{
		ID:               1,
		Code:             code,
		Seed:             seed,
		MineRate:         DefaultMineRate,
		AlgorithmVersion: MineAlgorithmHMACV1,
		Status:           MineSeasonStatusActive,
	}
}
