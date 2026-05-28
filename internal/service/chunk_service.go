package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"go-lobby/internal/dto/req"
	"go-lobby/internal/dto/res"
	"go-lobby/internal/model"
)

var (
	ErrInvalidChunkID  = errors.New("invalid chunk ID")
	ErrChunkNotFound   = errors.New("chunk not found")
	ErrNoActiveSeason  = errors.New("no active mine season")
	ErrChunkClosed     = errors.New("chunk is closed")
	ErrCellAlreadyOpen = errors.New("cell already opened")
)

type MineSeasonReader interface {
	GetActiveSeason(ctx context.Context) (*model.MineSeason, error)
}

type MineChunkStateStore interface {
	GetState(ctx context.Context, seasonID int64, chunkID string) (*model.MineChunkState, error)
	SaveState(ctx context.Context, state *model.MineChunkState) error
	ListClosedLeafChunkIDs(ctx context.Context, seasonID int64) (map[string]bool, error)
}

type ChunkService struct {
	seasonReader MineSeasonReader
	stateStore   MineChunkStateStore
	mineGen      *model.MineGenerator
	chunkLocks   sync.Map
}

func NewChunkService() *ChunkService {
	return &ChunkService{
		mineGen: model.NewMineGenerator(),
	}
}

func NewChunkServiceWithDeps(seasonReader MineSeasonReader, stateStore MineChunkStateStore) *ChunkService {
	return &ChunkService{
		seasonReader: seasonReader,
		stateStore:   stateStore,
		mineGen:      model.NewMineGenerator(),
	}
}

func (s *ChunkService) GetChunkSummary(ctx context.Context, rawChunkID string) (*res.ChunkSummaryResponse, error) {
	chunkID, err := parseDemoChunkID(rawChunkID)
	if err != nil {
		return nil, err
	}
	if !s.hasMineDeps() {
		return buildDemoChunkSummary(chunkID)
	}
	summary, err := buildBaseChunkSummary(chunkID)
	if err != nil {
		return nil, err
	}
	season, err := s.activeSeason(ctx)
	if err != nil {
		return nil, err
	}
	closedLeafIDs, err := s.stateStore.ListClosedLeafChunkIDs(ctx, season.ID)
	if err != nil {
		return nil, err
	}
	if chunkID.Z == model.ChunkMaxLevel {
		state, err := s.loadChunkState(ctx, season.ID, chunkID.String())
		if err != nil {
			return nil, err
		}
		applyMineStateToSummary(summary, state)
	}
	applyClosedLeafAggregate(summary, chunkID, closedLeafIDs)
	return summary, nil
}

func (s *ChunkService) GetChunkSnapshot(ctx context.Context, rawChunkID string) (*res.ChunkSnapshotResponse, error) {
	chunkID, err := parseDemoChunkID(rawChunkID)
	if err != nil {
		return nil, err
	}
	if s.hasMineDeps() {
		summary, err := buildDemoChunkSummary(chunkID)
		if err != nil {
			return nil, err
		}
		if chunkID.Z != model.ChunkMaxLevel {
			return &res.ChunkSnapshotResponse{
				ChunkID:     chunkID.String(),
				Width:       model.ChunkSize,
				Height:      model.ChunkSize,
				Closed:      summary.Closed,
				Version:     summary.Version,
				OpenedCells: nil,
			}, nil
		}
		season, err := s.activeSeason(ctx)
		if err != nil {
			return nil, err
		}
		state, err := s.loadChunkState(ctx, season.ID, chunkID.String())
		if err != nil {
			return nil, err
		}
		return buildSnapshotFromState(chunkID, state), nil
	}
	summary, err := buildDemoChunkSummary(chunkID)
	if err != nil {
		return nil, err
	}

	return &res.ChunkSnapshotResponse{
		ChunkID:     chunkID.String(),
		Width:       model.ChunkSize,
		Height:      model.ChunkSize,
		Closed:      summary.Closed,
		Version:     summary.Version,
		OpenedCells: buildDemoOpenedCells(chunkID),
	}, nil
}

func (s *ChunkService) ListChunks(ctx context.Context, level int, bbox model.ChunkBounds) (*res.ChunkListResponse, error) {
	minX, minY, maxX, maxY, err := model.VisibleChunkRange(level, bbox)
	if err != nil {
		return nil, ErrInvalidChunkID
	}
	var season *model.MineSeason
	var closedLeafIDs map[string]bool
	if s.hasMineDeps() {
		season, err = s.activeSeason(ctx)
		if err != nil {
			return nil, err
		}
		closedLeafIDs, err = s.stateStore.ListClosedLeafChunkIDs(ctx, season.ID)
		if err != nil {
			return nil, err
		}
	}

	chunks := make([]res.ChunkSummaryResponse, 0, (maxX-minX+1)*(maxY-minY+1))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			chunkID := model.ChunkID{
				Region: "cn",
				Z:      level,
				X:      x,
				Y:      y,
			}
			summary, err := buildDemoChunkSummary(chunkID)
			if err != nil {
				return nil, err
			}
			if s.hasMineDeps() {
				summary, err = buildBaseChunkSummary(chunkID)
				if err != nil {
					return nil, err
				}
				state, err := s.loadChunkState(ctx, season.ID, summary.ChunkID)
				if err != nil {
					return nil, err
				}
				if level == model.ChunkMaxLevel {
					applyMineStateToSummary(summary, state)
				}
				applyClosedLeafAggregate(summary, chunkID, closedLeafIDs)
			}
			chunks = append(chunks, *summary)
		}
	}

	return &res.ChunkListResponse{
		Region: "cn",
		Level:  level,
		BBox:   bbox,
		Chunks: chunks,
	}, nil
}

func (s *ChunkService) OpenCell(ctx context.Context, userID int64, rawChunkID string, openReq *req.OpenMineCellRequest) (*res.OpenMineCellResponse, error) {
	if !s.hasMineDeps() {
		return nil, ErrNoActiveSeason
	}
	chunkID, err := parsePlayableChunkID(rawChunkID)
	if err != nil {
		return nil, err
	}
	unlock := s.lockChunk(chunkID.String())
	defer unlock()
	index, err := model.ChunkCellIndex(openReq.X, openReq.Y)
	if err != nil {
		return nil, ErrInvalidChunkID
	}
	season, err := s.activeSeason(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.loadOrNewChunkState(ctx, season.ID, chunkID.String())
	if err != nil {
		return nil, err
	}
	if state.Closed {
		return nil, ErrChunkClosed
	}
	if opened, ok := state.OpenedCells[index]; ok {
		return &res.OpenMineCellResponse{
			ChunkID:       chunkID.String(),
			X:             opened.X,
			Y:             opened.Y,
			Index:         opened.Index,
			Mine:          false,
			Closed:        state.Closed,
			AdjacentMines: opened.AdjacentMines,
			Version:       state.Version,
			OpenedAt:      opened.OpenedAt,
			OpenedCells:   []res.OpenedCellResponse{openedCellResponse(opened)},
		}, nil
	}

	now := time.Now()
	isMine, err := s.mineGen.IsMine(season, chunkID, openReq.X, openReq.Y)
	if err != nil {
		return nil, err
	}
	adjacentMines := 0
	if isMine {
		state.Closed = true
		state.ClosedBy = userID
		state.ClosedAt = &now
	} else {
		adjacentMines, err = s.mineGen.AdjacentMineCount(season, chunkID, openReq.X, openReq.Y)
		if err != nil {
			return nil, err
		}
		if adjacentMines == 0 {
			if err := s.openZeroArea(season, state, chunkID, openReq.X, openReq.Y, userID, now); err != nil {
				return nil, err
			}
		} else {
			openCellInState(state, openReq.X, openReq.Y, index, userID, now, adjacentMines)
		}
	}
	state.Version++
	if err := s.stateStore.SaveState(ctx, state); err != nil {
		return nil, err
	}

	return &res.OpenMineCellResponse{
		ChunkID:       chunkID.String(),
		X:             openReq.X,
		Y:             openReq.Y,
		Index:         index,
		Mine:          isMine,
		Closed:        state.Closed,
		AdjacentMines: adjacentMines,
		Version:       state.Version,
		OpenedAt:      now,
		OpenedCells:   openedCellsResponse(state.OpenedCells),
	}, nil
}

func (s *ChunkService) openZeroArea(
	season *model.MineSeason,
	state *model.MineChunkState,
	chunkID model.ChunkID,
	startX int,
	startY int,
	userID int64,
	openedAt time.Time,
) error {
	type cell struct {
		x int
		y int
	}
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
			return ErrInvalidChunkID
		}
		if visited[index] {
			continue
		}
		visited[index] = true
		if _, ok := state.OpenedCells[index]; ok {
			continue
		}
		isMine, err := s.mineGen.IsMine(season, chunkID, current.x, current.y)
		if err != nil {
			return err
		}
		if isMine {
			continue
		}
		adjacentMines, err := s.mineGen.AdjacentMineCount(season, chunkID, current.x, current.y)
		if err != nil {
			return err
		}
		openCellInState(state, current.x, current.y, index, userID, openedAt, adjacentMines)
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
	return nil
}

func openCellInState(state *model.MineChunkState, x int, y int, index int, userID int64, openedAt time.Time, adjacentMines int) {
	state.OpenedCells[index] = model.MineOpenedCellSnapshot{
		X:             x,
		Y:             y,
		Index:         index,
		OpenedBy:      userID,
		OpenedAt:      openedAt,
		AdjacentMines: adjacentMines,
	}
	delete(state.FlaggedBy, index)
}

func (s *ChunkService) FlagCell(ctx context.Context, userID int64, rawChunkID string, flagReq *req.FlagMineCellRequest) (*res.FlagMineCellResponse, error) {
	if !s.hasMineDeps() {
		return nil, ErrNoActiveSeason
	}
	chunkID, err := parsePlayableChunkID(rawChunkID)
	if err != nil {
		return nil, err
	}
	unlock := s.lockChunk(chunkID.String())
	defer unlock()
	index, err := model.ChunkCellIndex(flagReq.X, flagReq.Y)
	if err != nil {
		return nil, ErrInvalidChunkID
	}
	season, err := s.activeSeason(ctx)
	if err != nil {
		return nil, err
	}
	state, err := s.loadOrNewChunkState(ctx, season.ID, chunkID.String())
	if err != nil {
		return nil, err
	}
	if state.Closed {
		return nil, ErrChunkClosed
	}
	if _, ok := state.OpenedCells[index]; ok {
		return nil, ErrCellAlreadyOpen
	}
	if flagReq.Flagged {
		state.FlaggedBy[index] = userID
	} else {
		delete(state.FlaggedBy, index)
	}
	state.Version++
	if err := s.stateStore.SaveState(ctx, state); err != nil {
		return nil, err
	}
	return &res.FlagMineCellResponse{
		ChunkID: chunkID.String(),
		X:       flagReq.X,
		Y:       flagReq.Y,
		Index:   index,
		Flagged: flagReq.Flagged,
		Version: state.Version,
	}, nil
}

func (s *ChunkService) hasMineDeps() bool {
	return s.seasonReader != nil && s.stateStore != nil && s.mineGen != nil
}

func (s *ChunkService) lockChunk(chunkID string) func() {
	lockValue, _ := s.chunkLocks.LoadOrStore(chunkID, &sync.Mutex{})
	mu := lockValue.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *ChunkService) activeSeason(ctx context.Context) (*model.MineSeason, error) {
	season, err := s.seasonReader.GetActiveSeason(ctx)
	if err != nil {
		return nil, err
	}
	if season == nil {
		return nil, ErrNoActiveSeason
	}
	return season, nil
}

func (s *ChunkService) loadChunkState(ctx context.Context, seasonID int64, chunkID string) (*model.MineChunkState, error) {
	state, err := s.stateStore.GetState(ctx, seasonID, chunkID)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (s *ChunkService) loadOrNewChunkState(ctx context.Context, seasonID int64, chunkID string) (*model.MineChunkState, error) {
	state, err := s.loadChunkState(ctx, seasonID, chunkID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return model.NewMineChunkState(seasonID, chunkID), nil
	}
	return state, nil
}

func applyMineStateToSummary(summary *res.ChunkSummaryResponse, state *model.MineChunkState) {
	if state == nil {
		summary.State = "normal"
		summary.Closed = false
		summary.OpenedCount = 0
		summary.Version = 1
		return
	}
	summary.Closed = state.Closed
	summary.Version = state.Version
	summary.OpenedCount = len(state.OpenedCells)
	switch {
	case state.Closed:
		summary.State = "closed"
	case len(state.OpenedCells) > 0:
		summary.State = "opening"
	default:
		summary.State = "normal"
	}
}

func applyClosedLeafAggregate(summary *res.ChunkSummaryResponse, chunkID model.ChunkID, closedLeafIDs map[string]bool) {
	totalLeafCount := totalLeafCountForLevel(chunkID.Z)
	closedLeafCount := countClosedLeaves(chunkID, closedLeafIDs)
	summary.TotalLeafCount = totalLeafCount
	summary.ClosedLeafCount = closedLeafCount
	if totalLeafCount > 0 {
		summary.ClosedRatio = float64(closedLeafCount) / float64(totalLeafCount)
	}

	if chunkID.Z == model.ChunkMaxLevel {
		summary.Closed = closedLeafCount == 1
	}
	switch {
	case closedLeafCount == totalLeafCount && totalLeafCount > 0:
		summary.State = "closed"
	case closedLeafCount > 0:
		summary.State = "closing"
	case summary.OpenedCount > 0:
		summary.State = "opening"
	default:
		summary.State = "normal"
	}
}

func countClosedLeaves(chunkID model.ChunkID, closedLeafIDs map[string]bool) int {
	if len(closedLeafIDs) == 0 {
		return 0
	}
	minX, minY, maxX, maxY := leafRangeForChunk(chunkID)
	count := 0
	for rawLeafID := range closedLeafIDs {
		leafID, err := model.ParseChunkID(rawLeafID)
		if err != nil || leafID.Region != chunkID.Region || leafID.Z != model.ChunkMaxLevel {
			continue
		}
		if leafID.X >= minX && leafID.X < maxX && leafID.Y >= minY && leafID.Y < maxY {
			count++
		}
	}
	return count
}

func leafRangeForChunk(chunkID model.ChunkID) (minX, minY, maxX, maxY int) {
	scale := 1 << (model.ChunkMaxLevel - chunkID.Z)
	minX = chunkID.X * scale
	minY = chunkID.Y * scale
	maxX = (chunkID.X + 1) * scale
	maxY = (chunkID.Y + 1) * scale
	return minX, minY, maxX, maxY
}

func totalLeafCountForLevel(level int) int {
	scale := 1 << (model.ChunkMaxLevel - level)
	return scale * scale
}

func buildSnapshotFromState(chunkID model.ChunkID, state *model.MineChunkState) *res.ChunkSnapshotResponse {
	snapshot := &res.ChunkSnapshotResponse{
		ChunkID: chunkID.String(),
		Width:   model.ChunkSize,
		Height:  model.ChunkSize,
		Version: 1,
	}
	if state == nil {
		return snapshot
	}
	snapshot.Closed = state.Closed
	snapshot.Version = state.Version
	snapshot.OpenedCells = make([]res.OpenedCellResponse, 0, len(state.OpenedCells))
	for _, cell := range state.OpenedCells {
		snapshot.OpenedCells = append(snapshot.OpenedCells, openedCellResponse(cell))
	}
	snapshot.FlaggedCells = flaggedCellsResponse(state.FlaggedBy)
	return snapshot
}

func openedCellsResponse(cells map[int]model.MineOpenedCellSnapshot) []res.OpenedCellResponse {
	responses := make([]res.OpenedCellResponse, 0, len(cells))
	for _, cell := range cells {
		responses = append(responses, openedCellResponse(cell))
	}
	return responses
}

func openedCellResponse(cell model.MineOpenedCellSnapshot) res.OpenedCellResponse {
	return res.OpenedCellResponse{
		X:             cell.X,
		Y:             cell.Y,
		Index:         cell.Index,
		AdjacentMines: cell.AdjacentMines,
		OpenedBy: res.CellOpenedByResponse{
			UserID: cell.OpenedBy,
		},
		OpenedAt: cell.OpenedAt,
	}
}

func flaggedCellsResponse(flaggedBy map[int]int64) []res.FlaggedCellResponse {
	responses := make([]res.FlaggedCellResponse, 0, len(flaggedBy))
	for index, userID := range flaggedBy {
		responses = append(responses, res.FlaggedCellResponse{
			X:     index % model.ChunkSize,
			Y:     index / model.ChunkSize,
			Index: index,
			FlaggedBy: res.CellFlaggedByResponse{
				UserID: userID,
			},
		})
	}
	return responses
}

func parseDemoChunkID(rawChunkID string) (model.ChunkID, error) {
	chunkID, err := model.ParseChunkID(rawChunkID)
	if err != nil {
		return model.ChunkID{}, ErrInvalidChunkID
	}
	if chunkID.Region != "cn" {
		return model.ChunkID{}, ErrChunkNotFound
	}
	gridSize, err := model.ChunkGridSize(chunkID.Z)
	if err != nil {
		return model.ChunkID{}, ErrInvalidChunkID
	}
	if chunkID.X >= gridSize || chunkID.Y >= gridSize {
		return model.ChunkID{}, ErrChunkNotFound
	}
	return chunkID, nil
}

func parsePlayableChunkID(rawChunkID string) (model.ChunkID, error) {
	chunkID, err := parseDemoChunkID(rawChunkID)
	if err != nil {
		return model.ChunkID{}, err
	}
	if err := model.EnsurePlayableChunk(chunkID); err != nil {
		return model.ChunkID{}, ErrInvalidChunkID
	}
	return chunkID, nil
}

func buildDemoChunkSummary(chunkID model.ChunkID) (*res.ChunkSummaryResponse, error) {
	summary, err := buildBaseChunkSummary(chunkID)
	if err != nil {
		return nil, err
	}
	state := demoChunkState(chunkID)
	summary.State = state
	summary.Closed = state == "closed"
	summary.Version = int64(1 + chunkID.Z*10000 + chunkID.Y*100 + chunkID.X)
	summary.OpenedCount = demoOpenedCount(chunkID)
	return summary, nil
}

func buildBaseChunkSummary(chunkID model.ChunkID) (*res.ChunkSummaryResponse, error) {
	bounds, err := model.ChunkBoundsFor(chunkID.Z, chunkID.X, chunkID.Y)
	if err != nil {
		return nil, ErrInvalidChunkID
	}
	return &res.ChunkSummaryResponse{
		ChunkID:        chunkID.String(),
		Region:         chunkID.Region,
		Z:              chunkID.Z,
		Level:          chunkID.Z,
		X:              chunkID.X,
		Y:              chunkID.Y,
		Bounds:         bounds,
		Width:          model.ChunkSize,
		Height:         model.ChunkSize,
		State:          "normal",
		Version:        1,
		TotalLeafCount: totalLeafCountForLevel(chunkID.Z),
	}, nil
}

func demoChunkState(chunkID model.ChunkID) string {
	score := (chunkID.X*31 + chunkID.Y*17 + chunkID.Z*13) % 23
	switch {
	case score == 0 || score == 7:
		return "closed"
	case score == 3 || score == 11:
		return "hot"
	case score == 5 || score == 19:
		return "opening"
	default:
		return "normal"
	}
}

func demoOpenedCount(chunkID model.ChunkID) int {
	if chunkID.Z == model.ChunkMaxLevel {
		return len(buildDemoOpenedCells(chunkID))
	}
	return 80 + ((chunkID.X*43 + chunkID.Y*29 + chunkID.Z*97) % 600)
}

func buildDemoOpenedCells(chunkID model.ChunkID) []res.OpenedCellResponse {
	players := []res.CellOpenedByResponse{
		{UserID: 1001, Nickname: "演示玩家A"},
		{UserID: 1002, Nickname: "演示玩家B"},
	}
	openedAt := time.Date(2026, 5, 25, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	cells := make([]res.OpenedCellResponse, 0, 256)
	offsetX := (chunkID.X * 11) % 28
	offsetY := (chunkID.Y * 7) % 28

	for y := 42 + offsetY; y < 58+offsetY; y++ {
		for x := 38 + offsetX; x < 54+offsetX; x++ {
			index, _ := model.ChunkCellIndex(x, y)
			cells = append(cells, res.OpenedCellResponse{
				X:        x,
				Y:        y,
				Index:    index,
				OpenedBy: players[(x+y)%len(players)],
				OpenedAt: openedAt,
			})
		}
	}

	return cells
}
