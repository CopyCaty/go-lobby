package service

import (
	"context"
	"errors"
	"go-lobby/internal/dto/req"
	"go-lobby/internal/model"
	"testing"
)

func TestChunkServiceOpenCellWritesOpenedState(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findMineCell(t, season, chunkID, false)

	resp, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if resp.Mine || resp.Closed {
		t.Fatalf("safe cell should not close chunk: %+v", resp)
	}
	state, err := store.GetState(context.Background(), season.ID, chunkID.String())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if len(state.OpenedCells) != 1 {
		t.Fatalf("unexpected opened cell count: %d", len(state.OpenedCells))
	}
}

func TestChunkServiceOpenMineClosesChunk(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findMineCell(t, season, chunkID, true)

	resp, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if !resp.Mine || !resp.Closed {
		t.Fatalf("mine cell should close chunk: %+v", resp)
	}
	if _, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: 0, Y: 0}); !errors.Is(err, ErrChunkClosed) {
		t.Fatalf("expected ErrChunkClosed, got %v", err)
	}
}

func TestChunkServiceOpenCellIsIdempotent(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findMineCell(t, season, chunkID, false)

	first, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	second, err := svc.OpenCell(context.Background(), 1002, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if first.Index != second.Index || first.Version != second.Version {
		t.Fatalf("repeat open should return existing state: first=%+v second=%+v", first, second)
	}
}

func TestChunkServiceOpenCellVersionIncrements(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	firstX, firstY := findNumberCell(t, season, chunkID, nil)
	opened := map[int]bool{}
	firstIndex, err := model.ChunkCellIndex(firstX, firstY)
	if err != nil {
		t.Fatalf("ChunkCellIndex returned error: %v", err)
	}
	opened[firstIndex] = true
	secondX, secondY := findNumberCell(t, season, chunkID, opened)

	first, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: firstX, Y: firstY})
	if err != nil {
		t.Fatalf("OpenCell first returned error: %v", err)
	}
	second, err := svc.OpenCell(context.Background(), 1002, chunkID.String(), &req.OpenMineCellRequest{X: secondX, Y: secondY})
	if err != nil {
		t.Fatalf("OpenCell second returned error: %v", err)
	}
	if second.Version != first.Version+1 {
		t.Fatalf("version should increment by one: first=%d second=%d", first.Version, second.Version)
	}
}

func TestChunkServiceOpenZeroCellCascades(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findZeroAdjacentCell(t, season, chunkID)

	resp, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if resp.Mine || resp.AdjacentMines != 0 {
		t.Fatalf("expected zero safe cell, got %+v", resp)
	}
	if len(resp.OpenedCells) <= 1 {
		t.Fatalf("zero cell should cascade open multiple cells, got %d", len(resp.OpenedCells))
	}
	state, err := store.GetState(context.Background(), season.ID, chunkID.String())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if len(state.OpenedCells) != len(resp.OpenedCells) {
		t.Fatalf("response/state opened count mismatch: response=%d state=%d", len(resp.OpenedCells), len(state.OpenedCells))
	}
}

func TestChunkServiceSnapshotDoesNotExposeMines(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findMineCell(t, season, chunkID, false)
	if _, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y}); err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}

	snapshot, err := svc.GetChunkSnapshot(context.Background(), chunkID.String())
	if err != nil {
		t.Fatalf("GetChunkSnapshot returned error: %v", err)
	}
	if len(snapshot.OpenedCells) != 1 {
		t.Fatalf("unexpected opened cell count: %d", len(snapshot.OpenedCells))
	}
}

func TestChunkServiceLeafClosedAggregate(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 8, Y: 20}
	store.items[chunkID.String()] = &model.MineChunkState{
		SeasonID:    season.ID,
		ChunkID:     chunkID.String(),
		Closed:      true,
		Version:     2,
		OpenedCells: make(map[int]model.MineOpenedCellSnapshot),
		FlaggedBy:   make(map[int]int64),
	}
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)

	summary, err := svc.GetChunkSummary(context.Background(), chunkID.String())
	if err != nil {
		t.Fatalf("GetChunkSummary returned error: %v", err)
	}
	if !summary.Closed || summary.State != "closed" || summary.ClosedLeafCount != 1 || summary.TotalLeafCount != 1 || summary.ClosedRatio != 1 {
		t.Fatalf("unexpected leaf aggregate: %+v", summary)
	}
}

func TestChunkServiceParentClosedLeafAggregate(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	for _, rawChunkID := range []string{"cn:6:8:20", "cn:6:0:0"} {
		store.items[rawChunkID] = &model.MineChunkState{
			SeasonID:    season.ID,
			ChunkID:     rawChunkID,
			Closed:      true,
			Version:     2,
			OpenedCells: make(map[int]model.MineOpenedCellSnapshot),
			FlaggedBy:   make(map[int]int64),
		}
	}
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)

	level5, err := svc.GetChunkSummary(context.Background(), "cn:5:4:10")
	if err != nil {
		t.Fatalf("GetChunkSummary level5 returned error: %v", err)
	}
	if level5.Closed || level5.State != "closing" || level5.ClosedLeafCount != 1 || level5.TotalLeafCount != 4 || level5.ClosedRatio != 0.25 {
		t.Fatalf("unexpected level5 aggregate: %+v", level5)
	}

	level4, err := svc.GetChunkSummary(context.Background(), "cn:4:2:5")
	if err != nil {
		t.Fatalf("GetChunkSummary level4 returned error: %v", err)
	}
	if level4.Closed || level4.State != "closing" || level4.ClosedLeafCount != 1 || level4.TotalLeafCount != 16 || level4.ClosedRatio != 0.0625 {
		t.Fatalf("unexpected level4 aggregate: %+v", level4)
	}

	level0, err := svc.GetChunkSummary(context.Background(), "cn:0:0:0")
	if err != nil {
		t.Fatalf("GetChunkSummary level0 returned error: %v", err)
	}
	if level0.Closed || level0.State != "closing" || level0.ClosedLeafCount != 2 || level0.TotalLeafCount != 4096 {
		t.Fatalf("unexpected level0 aggregate: %+v", level0)
	}
}

type staticMineSeasonReader struct {
	season *model.MineSeason
}

func (r staticMineSeasonReader) GetActiveSeason(ctx context.Context) (*model.MineSeason, error) {
	return r.season, nil
}

type memoryMineChunkStateStore struct {
	items map[string]*model.MineChunkState
}

func newMemoryMineChunkStateStore() *memoryMineChunkStateStore {
	return &memoryMineChunkStateStore{items: make(map[string]*model.MineChunkState)}
}

func (s *memoryMineChunkStateStore) GetState(ctx context.Context, seasonID int64, chunkID string) (*model.MineChunkState, error) {
	return s.items[chunkID], nil
}

func (s *memoryMineChunkStateStore) SaveState(ctx context.Context, state *model.MineChunkState) error {
	s.items[state.ChunkID] = state
	return nil
}

func (s *memoryMineChunkStateStore) ListClosedLeafChunkIDs(ctx context.Context, seasonID int64) (map[string]bool, error) {
	closed := make(map[string]bool)
	for chunkID, state := range s.items {
		if state.SeasonID == seasonID && state.Closed {
			closed[chunkID] = true
		}
	}
	return closed, nil
}

func testServiceMineSeason() *model.MineSeason {
	return &model.MineSeason{
		ID:               1,
		Code:             "s1",
		Seed:             "seed-a",
		MineRate:         model.DefaultMineRate,
		AlgorithmVersion: model.MineAlgorithmHMACV1,
		Status:           model.MineSeasonStatusActive,
	}
}

func findMineCell(t *testing.T, season *model.MineSeason, chunkID model.ChunkID, wantMine bool) (int, int) {
	t.Helper()
	gen := model.NewMineGenerator()
	for y := 0; y < model.ChunkSize; y++ {
		for x := 0; x < model.ChunkSize; x++ {
			isMine, err := gen.IsMine(season, chunkID, x, y)
			if err != nil {
				t.Fatalf("IsMine returned error: %v", err)
			}
			if isMine == wantMine {
				return x, y
			}
		}
	}
	t.Fatalf("could not find cell with mine=%v", wantMine)
	return 0, 0
}

func findNumberCell(t *testing.T, season *model.MineSeason, chunkID model.ChunkID, excluded map[int]bool) (int, int) {
	t.Helper()
	gen := model.NewMineGenerator()
	for y := 0; y < model.ChunkSize; y++ {
		for x := 0; x < model.ChunkSize; x++ {
			index, err := model.ChunkCellIndex(x, y)
			if err != nil {
				t.Fatalf("ChunkCellIndex returned error: %v", err)
			}
			if excluded[index] {
				continue
			}
			isMine, err := gen.IsMine(season, chunkID, x, y)
			if err != nil {
				t.Fatalf("IsMine returned error: %v", err)
			}
			if isMine {
				continue
			}
			adjacentMines, err := gen.AdjacentMineCount(season, chunkID, x, y)
			if err != nil {
				t.Fatalf("AdjacentMineCount returned error: %v", err)
			}
			if adjacentMines > 0 {
				return x, y
			}
		}
	}
	t.Fatal("could not find safe number cell")
	return 0, 0
}

func findZeroAdjacentCell(t *testing.T, season *model.MineSeason, chunkID model.ChunkID) (int, int) {
	t.Helper()
	gen := model.NewMineGenerator()
	for y := 0; y < model.ChunkSize; y++ {
		for x := 0; x < model.ChunkSize; x++ {
			isMine, err := gen.IsMine(season, chunkID, x, y)
			if err != nil {
				t.Fatalf("IsMine returned error: %v", err)
			}
			if isMine {
				continue
			}
			adjacentMines, err := gen.AdjacentMineCount(season, chunkID, x, y)
			if err != nil {
				t.Fatalf("AdjacentMineCount returned error: %v", err)
			}
			if adjacentMines == 0 {
				return x, y
			}
		}
	}
	t.Fatal("could not find zero adjacent cell")
	return 0, 0
}
