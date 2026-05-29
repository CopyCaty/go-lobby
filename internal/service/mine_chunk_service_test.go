package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"go-lobby/internal/dto/req"
	"go-lobby/internal/matchqueue"
	"go-lobby/internal/model"
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

func TestChunkServiceFirstMineOpenIsCanceled(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findMineCell(t, season, chunkID, true)

	resp, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if !resp.Mine || resp.Closed || !resp.Canceled {
		t.Fatalf("first mine open should be canceled without closing chunk: %+v", resp)
	}
	if resp.Version != 1 {
		t.Fatalf("canceled open should keep initial version, got %d", resp.Version)
	}
	state, err := store.GetState(context.Background(), season.ID, chunkID.String())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if state != nil {
		t.Fatalf("canceled first mine open should not save chunk state: %+v", state)
	}
}

func TestChunkServiceOpenMineClosesChunkAfterPlayerOpenedSafeCell(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	closureDuration := 30 * time.Second
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store, closureDuration)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	safeX, safeY := findMineCell(t, season, chunkID, false)
	mineX, mineY := findMineCell(t, season, chunkID, true)

	if _, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: safeX, Y: safeY}); err != nil {
		t.Fatalf("OpenCell safe returned error: %v", err)
	}
	resp, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: mineX, Y: mineY})
	if err != nil {
		t.Fatalf("OpenCell mine returned error: %v", err)
	}
	if !resp.Mine || !resp.Closed || resp.Canceled {
		t.Fatalf("mine open after safe cell should close chunk: %+v", resp)
	}
	if resp.ClosedAt == nil || resp.ClosedUntil == nil {
		t.Fatalf("closed response should include closed_at and closed_until: %+v", resp)
	}
	if got := resp.ClosedUntil.Sub(*resp.ClosedAt); got != closureDuration {
		t.Fatalf("unexpected closure duration: got %v want %v", got, closureDuration)
	}
	if _, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: 0, Y: 0}); !errors.Is(err, ErrChunkClosed) {
		t.Fatalf("expected ErrChunkClosed, got %v", err)
	}
	if _, err := svc.FlagCell(context.Background(), 1001, chunkID.String(), &req.FlagMineCellRequest{X: 0, Y: 0, Flagged: true}); !errors.Is(err, ErrChunkClosed) {
		t.Fatalf("expected ErrChunkClosed for flag, got %v", err)
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

func TestChunkServiceOpenFlaggedCellReturnsError(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findNumberCell(t, season, chunkID, nil)

	flagResp, err := svc.FlagCell(context.Background(), 1001, chunkID.String(), &req.FlagMineCellRequest{X: x, Y: y, Flagged: true})
	if err != nil {
		t.Fatalf("FlagCell returned error: %v", err)
	}
	if _, err := svc.OpenCell(context.Background(), 1002, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y}); !errors.Is(err, ErrCellFlagged) {
		t.Fatalf("expected ErrCellFlagged, got %v", err)
	}

	state, err := store.GetState(context.Background(), season.ID, chunkID.String())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if state.Version != flagResp.Version {
		t.Fatalf("flagged open should not increment version: flag=%d state=%d", flagResp.Version, state.Version)
	}
	if _, ok := state.FlaggedBy[flagResp.Index]; !ok {
		t.Fatalf("flagged open should keep flag at index %d", flagResp.Index)
	}
	if _, ok := state.OpenedCells[flagResp.Index]; ok {
		t.Fatalf("flagged open should not open index %d", flagResp.Index)
	}
}

func TestChunkServiceOpenZeroAreaSkipsFlaggedCells(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	startX, startY := findZeroAdjacentCell(t, season, chunkID)
	flagX, flagY := findZeroCascadeCell(t, season, chunkID, startX, startY)
	flagIndex, err := model.ChunkCellIndex(flagX, flagY)
	if err != nil {
		t.Fatalf("ChunkCellIndex returned error: %v", err)
	}

	if _, err := svc.FlagCell(context.Background(), 1001, chunkID.String(), &req.FlagMineCellRequest{X: flagX, Y: flagY, Flagged: true}); err != nil {
		t.Fatalf("FlagCell returned error: %v", err)
	}
	resp, err := svc.OpenCell(context.Background(), 1002, chunkID.String(), &req.OpenMineCellRequest{X: startX, Y: startY})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if resp.Mine || resp.AdjacentMines != 0 {
		t.Fatalf("expected zero safe cell, got %+v", resp)
	}

	state, err := store.GetState(context.Background(), season.ID, chunkID.String())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if _, ok := state.FlaggedBy[flagIndex]; !ok {
		t.Fatalf("zero area open should keep flag at index %d", flagIndex)
	}
	if _, ok := state.OpenedCells[flagIndex]; ok {
		t.Fatalf("zero area open should not open flagged index %d", flagIndex)
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

func TestChunkServiceSnapshotReturnsFlaggedCells(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}

	flagResp, err := svc.FlagCell(context.Background(), 1001, chunkID.String(), &req.FlagMineCellRequest{X: 7, Y: 9, Flagged: true})
	if err != nil {
		t.Fatalf("FlagCell returned error: %v", err)
	}
	snapshot, err := svc.GetChunkSnapshot(context.Background(), chunkID.String())
	if err != nil {
		t.Fatalf("GetChunkSnapshot returned error: %v", err)
	}
	if len(snapshot.FlaggedCells) != 1 {
		t.Fatalf("unexpected flagged cell count: %d", len(snapshot.FlaggedCells))
	}
	flagged := snapshot.FlaggedCells[0]
	if flagged.X != 7 || flagged.Y != 9 || flagged.Index != flagResp.Index {
		t.Fatalf("unexpected flagged cell: %+v", flagged)
	}
	if flagged.FlaggedBy.UserID != 1001 {
		t.Fatalf("unexpected flagged_by: %+v", flagged.FlaggedBy)
	}
}

func TestChunkServiceSnapshotOmitsCanceledFlaggedCells(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}

	if _, err := svc.FlagCell(context.Background(), 1001, chunkID.String(), &req.FlagMineCellRequest{X: 7, Y: 9, Flagged: true}); err != nil {
		t.Fatalf("FlagCell true returned error: %v", err)
	}
	if _, err := svc.FlagCell(context.Background(), 1001, chunkID.String(), &req.FlagMineCellRequest{X: 7, Y: 9, Flagged: false}); err != nil {
		t.Fatalf("FlagCell false returned error: %v", err)
	}
	snapshot, err := svc.GetChunkSnapshot(context.Background(), chunkID.String())
	if err != nil {
		t.Fatalf("GetChunkSnapshot returned error: %v", err)
	}
	if len(snapshot.FlaggedCells) != 0 {
		t.Fatalf("unexpected flagged cell count: %d", len(snapshot.FlaggedCells))
	}
}

func TestChunkServiceLazyReopensExpiredClosedChunk(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 8, Y: 20}
	closedAt := time.Now().Add(-2 * time.Minute)
	closedUntil := time.Now().Add(-time.Minute)
	store.items[chunkID.String()] = &model.MineChunkState{
		SeasonID:    season.ID,
		ChunkID:     chunkID.String(),
		Closed:      true,
		ClosedBy:    1001,
		ClosedAt:    &closedAt,
		ClosedUntil: &closedUntil,
		Version:     2,
		OpenedCells: make(map[int]model.MineOpenedCellSnapshot),
		FlaggedBy:   make(map[int]int64),
	}
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store, 30*time.Second)

	snapshot, err := svc.GetChunkSnapshot(context.Background(), chunkID.String())
	if err != nil {
		t.Fatalf("GetChunkSnapshot returned error: %v", err)
	}
	if snapshot.Closed || snapshot.ClosedAt != nil || snapshot.ClosedUntil != nil {
		t.Fatalf("expired chunk should reopen in snapshot: %+v", snapshot)
	}
	state := store.items[chunkID.String()]
	if state.Closed || state.ClosedAt != nil || state.ClosedUntil != nil || state.ClosedBy != 0 {
		t.Fatalf("expired chunk state should be reopened: %+v", state)
	}
	if state.Version != 3 {
		t.Fatalf("reopen should increment version: got %d", state.Version)
	}
	if _, err := svc.FlagCell(context.Background(), 1001, chunkID.String(), &req.FlagMineCellRequest{X: 7, Y: 9, Flagged: true}); err != nil {
		t.Fatalf("FlagCell after reopen returned error: %v", err)
	}
}

func TestChunkServiceExpiredClosedSummaryReturnsOpeningState(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 8, Y: 20}
	closedAt := time.Now().Add(-2 * time.Minute)
	closedUntil := time.Now().Add(-time.Minute)
	index, err := model.ChunkCellIndex(7, 9)
	if err != nil {
		t.Fatalf("ChunkCellIndex returned error: %v", err)
	}
	store.items[chunkID.String()] = &model.MineChunkState{
		SeasonID:    season.ID,
		ChunkID:     chunkID.String(),
		Closed:      true,
		ClosedBy:    1001,
		ClosedAt:    &closedAt,
		ClosedUntil: &closedUntil,
		Version:     2,
		OpenedCells: map[int]model.MineOpenedCellSnapshot{
			index: {X: 7, Y: 9, Index: index, OpenedBy: 1001, OpenedAt: closedAt, AdjacentMines: 1},
		},
		FlaggedBy: make(map[int]int64),
	}
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store, 30*time.Second)

	summary, err := svc.GetChunkSummary(context.Background(), chunkID.String())
	if err != nil {
		t.Fatalf("GetChunkSummary returned error: %v", err)
	}
	if summary.Closed || summary.State != "opening" || summary.ClosedAt != nil || summary.ClosedUntil != nil {
		t.Fatalf("unexpected summary after lazy reopen: %+v", summary)
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

func TestChunkServiceConcurrentOpenSameChunkSerializesState(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	cellCount := 8
	cells := make([][2]int, 0, cellCount)
	excluded := make(map[int]bool)
	for len(cells) < cellCount {
		x, y := findNumberCell(t, season, chunkID, excluded)
		index, err := model.ChunkCellIndex(x, y)
		if err != nil {
			t.Fatalf("ChunkCellIndex returned error: %v", err)
		}
		excluded[index] = true
		cells = append(cells, [2]int{x, y})
	}

	var wg sync.WaitGroup
	errs := make(chan error, cellCount)
	for i, cell := range cells {
		wg.Add(1)
		go func(userID int64, x int, y int) {
			defer wg.Done()
			_, err := svc.OpenCell(context.Background(), userID, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
			errs <- err
		}(int64(1000+i), cell[0], cell[1])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("OpenCell returned error: %v", err)
		}
	}

	state, err := store.GetState(context.Background(), season.ID, chunkID.String())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if len(state.OpenedCells) != cellCount {
		t.Fatalf("unexpected opened cell count: got %d want %d", len(state.OpenedCells), cellCount)
	}
	if state.Version != int64(cellCount+1) {
		t.Fatalf("unexpected version: got %d want %d", state.Version, cellCount+1)
	}
}

func TestChunkServiceConcurrentOpenSameCellIsIdempotent(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findNumberCell(t, season, chunkID, nil)
	requestCount := 12

	var wg sync.WaitGroup
	errs := make(chan error, requestCount)
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(userID int64) {
			defer wg.Done()
			_, err := svc.OpenCell(context.Background(), userID, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
			errs <- err
		}(int64(1000 + i))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("OpenCell returned error: %v", err)
		}
	}

	state, err := store.GetState(context.Background(), season.ID, chunkID.String())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if len(state.OpenedCells) != 1 {
		t.Fatalf("same cell should only be opened once, got %d", len(state.OpenedCells))
	}
	if state.Version != 2 {
		t.Fatalf("same cell repeated open should only increment once, got version %d", state.Version)
	}
}

func TestChunkServiceConcurrentOpenDifferentChunks(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunks := []model.ChunkID{
		{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20},
		{Region: "cn", Z: model.ChunkMaxLevel, X: 11, Y: 20},
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(chunks))
	for i, chunkID := range chunks {
		x, y := findNumberCell(t, season, chunkID, nil)
		wg.Add(1)
		go func(userID int64, chunkID model.ChunkID, x int, y int) {
			defer wg.Done()
			_, err := svc.OpenCell(context.Background(), userID, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
			errs <- err
		}(int64(1000+i), chunkID, x, y)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("OpenCell returned error: %v", err)
		}
	}

	for _, chunkID := range chunks {
		state, err := store.GetState(context.Background(), season.ID, chunkID.String())
		if err != nil {
			t.Fatalf("GetState returned error: %v", err)
		}
		if state == nil || len(state.OpenedCells) != 1 || state.Version != 2 {
			t.Fatalf("unexpected state for chunk %s: %+v", chunkID.String(), state)
		}
	}
}

func TestChunkServiceCanceledContextDoesNotCreateWorkerOrState(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findNumberCell(t, season, chunkID, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.OpenCell(ctx, 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	state, err := store.GetState(context.Background(), season.ID, chunkID.String())
	if err != nil {
		t.Fatalf("GetState returned error: %v", err)
	}
	if state != nil {
		t.Fatalf("canceled request should not create state: %+v", state)
	}
	if got := svc.chunkWorkerCount(); got != 0 {
		t.Fatalf("canceled request should not create worker, got %d", got)
	}
}

func TestChunkServiceWorkerExitsAfterIdleTimeout(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	svc.workerIdleTimeout = 10 * time.Millisecond
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	x, y := findNumberCell(t, season, chunkID, nil)

	if _, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y}); err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if got := svc.chunkWorkerCount(); got != 1 {
		t.Fatalf("expected one worker after open, got %d", got)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if svc.chunkWorkerCount() == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("worker did not exit after idle timeout, workers=%d", svc.chunkWorkerCount())
}

func TestChunkServiceMineMatchRejectsUsersOutsideLockedChunk(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	matchStore := newMemoryMineMatchStore()
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	setupMineMatchState(t, matchStore, season.ID, 9001, chunkID.String())
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	svc.SetMineMatchService(NewMineMatchService(staticMineSeasonReader{season: season}, store, matchStore, nil))
	x, y := findNumberCell(t, season, chunkID, nil)

	if _, err := svc.OpenCell(context.Background(), 3003, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y}); !errors.Is(err, ErrMineMatchChunkOccupied) {
		t.Fatalf("expected ErrMineMatchChunkOccupied, got %v", err)
	}
}

func TestChunkServiceMineMatchPlayerCanOnlyOperateAssignedChunk(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	matchStore := newMemoryMineMatchStore()
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	otherChunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 11, Y: 20}
	setupMineMatchState(t, matchStore, season.ID, 9001, chunkID.String())
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	svc.SetMineMatchService(NewMineMatchService(staticMineSeasonReader{season: season}, store, matchStore, nil))
	x, y := findNumberCell(t, season, otherChunkID, nil)

	if _, err := svc.OpenCell(context.Background(), 1001, otherChunkID.String(), &req.OpenMineCellRequest{X: x, Y: y}); !errors.Is(err, ErrMineMatchWrongChunk) {
		t.Fatalf("expected ErrMineMatchWrongChunk, got %v", err)
	}
}

func TestChunkServiceMineMatchScoresNewOpenedCells(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	matchStore := newMemoryMineMatchStore()
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	setupMineMatchState(t, matchStore, season.ID, 9001, chunkID.String())
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	svc.SetMineMatchService(NewMineMatchService(staticMineSeasonReader{season: season}, store, matchStore, nil))
	x, y := findNumberCell(t, season, chunkID, nil)

	resp, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if resp.NewOpenedCount != 1 {
		t.Fatalf("unexpected new opened count: %d", resp.NewOpenedCount)
	}
	if resp.MatchID != 9001 || resp.MatchScores[1001] != 1 {
		t.Fatalf("unexpected match response: %+v", resp)
	}
}

func TestChunkServiceMineMatchFirstMineOpenKeepsMatchOngoing(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	matchStore := newMemoryMineMatchStore()
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	setupMineMatchState(t, matchStore, season.ID, 9001, chunkID.String())
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	svc.SetMineMatchService(NewMineMatchService(staticMineSeasonReader{season: season}, store, matchStore, nil))
	x, y := findMineCell(t, season, chunkID, true)

	resp, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: x, Y: y})
	if err != nil {
		t.Fatalf("OpenCell returned error: %v", err)
	}
	if !resp.Canceled || resp.MatchFinished || resp.MatchScores[1001] != 0 {
		t.Fatalf("first mine should be protected without finishing match: %+v", resp)
	}
	active, err := matchStore.GetActiveMatchByUser(context.Background(), 1001)
	if err != nil {
		t.Fatalf("GetActiveMatchByUser returned error: %v", err)
	}
	if active == nil || active.Status != model.MineMatchStatusOngoing {
		t.Fatalf("match should stay ongoing: %+v", active)
	}
}

func TestChunkServiceMineMatchMineAfterSafeOpenFinishesMatch(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	matchStore := newMemoryMineMatchStore()
	chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 10, Y: 20}
	setupMineMatchState(t, matchStore, season.ID, 9001, chunkID.String())
	svc := NewChunkServiceWithDeps(staticMineSeasonReader{season: season}, store)
	svc.SetMineMatchService(NewMineMatchService(staticMineSeasonReader{season: season}, store, matchStore, nil))
	safeX, safeY := findNumberCell(t, season, chunkID, nil)
	mineX, mineY := findMineCell(t, season, chunkID, true)

	if _, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: safeX, Y: safeY}); err != nil {
		t.Fatalf("safe OpenCell returned error: %v", err)
	}
	resp, err := svc.OpenCell(context.Background(), 1001, chunkID.String(), &req.OpenMineCellRequest{X: mineX, Y: mineY})
	if err != nil {
		t.Fatalf("mine OpenCell returned error: %v", err)
	}
	if !resp.Mine || !resp.MatchFinished || resp.WinTeamNo == nil || *resp.WinTeamNo != 1 {
		t.Fatalf("mine after safe open should finish with opponent win: %+v", resp)
	}
	if locked, err := matchStore.GetChunkLock(context.Background(), season.ID, chunkID.String()); err != nil || locked != 0 {
		t.Fatalf("chunk lock should be released, locked=%d err=%v", locked, err)
	}
	if active, err := matchStore.GetActiveMatchByUser(context.Background(), 1001); err != nil || active != nil {
		t.Fatalf("active match should be cleared, active=%+v err=%v", active, err)
	}
}

func TestMineMatchServiceAssignChunkSkipsDirtyAndLockedChunks(t *testing.T) {
	season := testServiceMineSeason()
	store := newMemoryMineChunkStateStore()
	matchStore := newMemoryMineMatchStore()
	gridSize, err := model.ChunkGridSize(model.ChunkMaxLevel)
	if err != nil {
		t.Fatalf("ChunkGridSize returned error: %v", err)
	}
	cleanChunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 3, Y: 4}.String()
	lockedChunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: 5, Y: 6}.String()
	for y := 0; y < gridSize; y++ {
		for x := 0; x < gridSize; x++ {
			chunkID := model.ChunkID{Region: "cn", Z: model.ChunkMaxLevel, X: x, Y: y}.String()
			if chunkID == cleanChunkID || chunkID == lockedChunkID {
				continue
			}
			store.items[chunkID] = &model.MineChunkState{
				SeasonID:    season.ID,
				ChunkID:     chunkID,
				Version:     1,
				OpenedCells: make(map[int]model.MineOpenedCellSnapshot),
				FlaggedBy:   map[int]int64{0: 1001},
			}
		}
	}
	if locked, err := matchStore.TryLockChunk(context.Background(), season.ID, lockedChunkID, 7777); err != nil || !locked {
		t.Fatalf("TryLockChunk locked=%v err=%v", locked, err)
	}
	svc := NewMineMatchService(staticMineSeasonReader{season: season}, store, matchStore, nil)
	teams := []matchqueue.MatchedTeam{
		{TeamID: 0, UserIDs: []int64{1001}},
		{TeamID: 1, UserIDs: []int64{1002}},
	}

	chunkID, err := svc.AssignChunkForMatch(context.Background(), 9001, "room-test", teams)
	if err != nil {
		t.Fatalf("AssignChunkForMatch returned error: %v", err)
	}
	if chunkID != cleanChunkID {
		t.Fatalf("unexpected assigned chunk: got %s want %s", chunkID, cleanChunkID)
	}
	if locked, err := matchStore.GetChunkLock(context.Background(), season.ID, cleanChunkID); err != nil || locked != 9001 {
		t.Fatalf("clean chunk should be locked by match, locked=%d err=%v", locked, err)
	}
}

type staticMineSeasonReader struct {
	season *model.MineSeason
}

func (r staticMineSeasonReader) GetActiveSeason(ctx context.Context) (*model.MineSeason, error) {
	return r.season, nil
}

type memoryMineChunkStateStore struct {
	mu    sync.Mutex
	items map[string]*model.MineChunkState
}

func newMemoryMineChunkStateStore() *memoryMineChunkStateStore {
	return &memoryMineChunkStateStore{items: make(map[string]*model.MineChunkState)}
}

func (s *memoryMineChunkStateStore) GetState(ctx context.Context, seasonID int64, chunkID string) (*model.MineChunkState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.items[chunkID], nil
}

func (s *memoryMineChunkStateStore) SaveState(ctx context.Context, state *model.MineChunkState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[state.ChunkID] = state
	return nil
}

func (s *memoryMineChunkStateStore) ListClosedLeafChunkIDs(ctx context.Context, seasonID int64) (map[string]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	closed := make(map[string]bool)
	for chunkID, state := range s.items {
		if state.SeasonID == seasonID && state.Closed {
			closed[chunkID] = true
		}
	}
	return closed, nil
}

type memoryMineMatchStore struct {
	mu          sync.Mutex
	matches     map[int64]*model.MineMatchState
	activeUsers map[int64]int64
	locks       map[string]int64
}

func newMemoryMineMatchStore() *memoryMineMatchStore {
	return &memoryMineMatchStore{
		matches:     make(map[int64]*model.MineMatchState),
		activeUsers: make(map[int64]int64),
		locks:       make(map[string]int64),
	}
}

func (s *memoryMineMatchStore) GetMatchState(ctx context.Context, matchID int64) (*model.MineMatchState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.matches[matchID]
	if state == nil {
		return nil, nil
	}
	return cloneMineMatchState(state), nil
}

func (s *memoryMineMatchStore) SaveMatchState(ctx context.Context, state *model.MineMatchState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matches[state.MatchID] = cloneMineMatchState(state)
	return nil
}

func (s *memoryMineMatchStore) GetActiveMatchByUser(ctx context.Context, userID int64) (*model.MineMatchState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matchID := s.activeUsers[userID]
	if matchID == 0 {
		return nil, nil
	}
	state := s.matches[matchID]
	if state == nil || state.Status != model.MineMatchStatusOngoing {
		return nil, nil
	}
	return cloneMineMatchState(state), nil
}

func (s *memoryMineMatchStore) SetActiveMatchForUsers(ctx context.Context, state *model.MineMatchState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for userID := range state.UserTeams {
		s.activeUsers[userID] = state.MatchID
	}
	return nil
}

func (s *memoryMineMatchStore) DeleteActiveMatchForUsers(ctx context.Context, state *model.MineMatchState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for userID := range state.UserTeams {
		delete(s.activeUsers, userID)
	}
	return nil
}

func (s *memoryMineMatchStore) GetChunkLock(ctx context.Context, seasonID int64, chunkID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.locks[memoryMineChunkLockKey(seasonID, chunkID)], nil
}

func (s *memoryMineMatchStore) TryLockChunk(ctx context.Context, seasonID int64, chunkID string, matchID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryMineChunkLockKey(seasonID, chunkID)
	if s.locks[key] != 0 {
		return false, nil
	}
	s.locks[key] = matchID
	return true, nil
}

func (s *memoryMineMatchStore) UnlockChunk(ctx context.Context, seasonID int64, chunkID string, matchID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := memoryMineChunkLockKey(seasonID, chunkID)
	if s.locks[key] == matchID {
		delete(s.locks, key)
	}
	return nil
}

func setupMineMatchState(t *testing.T, store *memoryMineMatchStore, seasonID int64, matchID int64, chunkID string) {
	t.Helper()
	teams := []matchqueue.MatchedTeam{
		{TeamID: 0, UserIDs: []int64{1001}},
		{TeamID: 1, UserIDs: []int64{1002}},
	}
	state := buildMineMatchState(matchID, "room-test", seasonID, chunkID, teams)
	if err := store.SaveMatchState(context.Background(), state); err != nil {
		t.Fatalf("SaveMatchState returned error: %v", err)
	}
	if err := store.SetActiveMatchForUsers(context.Background(), state); err != nil {
		t.Fatalf("SetActiveMatchForUsers returned error: %v", err)
	}
	locked, err := store.TryLockChunk(context.Background(), seasonID, chunkID, matchID)
	if err != nil {
		t.Fatalf("TryLockChunk returned error: %v", err)
	}
	if !locked {
		t.Fatalf("expected chunk lock to be acquired")
	}
}

func memoryMineChunkLockKey(seasonID int64, chunkID string) string {
	return fmt.Sprintf("%d:%s", seasonID, chunkID)
}

func cloneMineMatchState(state *model.MineMatchState) *model.MineMatchState {
	cloned := *state
	cloned.Teams = make(map[int8][]int64, len(state.Teams))
	for teamNo, userIDs := range state.Teams {
		cloned.Teams[teamNo] = append([]int64(nil), userIDs...)
	}
	cloned.UserTeams = make(map[int64]int8, len(state.UserTeams))
	for userID, teamNo := range state.UserTeams {
		cloned.UserTeams[userID] = teamNo
	}
	cloned.PlayerScores = make(map[int64]int, len(state.PlayerScores))
	for userID, score := range state.PlayerScores {
		cloned.PlayerScores[userID] = score
	}
	cloned.LastScoreAt = make(map[int64]int64, len(state.LastScoreAt))
	for userID, lastScoreAt := range state.LastScoreAt {
		cloned.LastScoreAt[userID] = lastScoreAt
	}
	return &cloned
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

func findZeroCascadeCell(t *testing.T, season *model.MineSeason, chunkID model.ChunkID, startX int, startY int) (int, int) {
	t.Helper()
	type cell struct {
		x int
		y int
	}
	gen := model.NewMineGenerator()
	queue := []cell{{x: startX, y: startY}}
	visited := make(map[int]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.x < 0 || current.x >= model.ChunkSize || current.y < 0 || current.y >= model.ChunkSize {
			continue
		}
		index, err := model.ChunkCellIndex(current.x, current.y)
		if err != nil {
			t.Fatalf("ChunkCellIndex returned error: %v", err)
		}
		if visited[index] {
			continue
		}
		visited[index] = true

		isMine, err := gen.IsMine(season, chunkID, current.x, current.y)
		if err != nil {
			t.Fatalf("IsMine returned error: %v", err)
		}
		if isMine {
			continue
		}
		if current.x != startX || current.y != startY {
			return current.x, current.y
		}

		adjacentMines, err := gen.AdjacentMineCount(season, chunkID, current.x, current.y)
		if err != nil {
			t.Fatalf("AdjacentMineCount returned error: %v", err)
		}
		if adjacentMines != 0 {
			continue
		}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				queue = append(queue, cell{x: current.x + dx, y: current.y + dy})
			}
		}
	}
	t.Fatal("could not find cell in zero cascade")
	return 0, 0
}
