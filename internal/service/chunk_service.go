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
	ErrCellFlagged     = errors.New("cell is flagged")
)

const defaultChunkClosureDuration = 5 * time.Minute
const defaultChunkWorkerIdleTimeout = 5 * time.Minute
const defaultChunkWorkerQueueSize = 256

type MineSeasonReader interface {
	GetActiveSeason(ctx context.Context) (*model.MineSeason, error)
}

type MineChunkStateStore interface {
	GetState(ctx context.Context, seasonID int64, chunkID string) (*model.MineChunkState, error)
	SaveState(ctx context.Context, state *model.MineChunkState) error
	ListClosedLeafChunkIDs(ctx context.Context, seasonID int64) (map[string]bool, error)
}

type ChunkService struct {
	seasonReader      MineSeasonReader
	stateStore        MineChunkStateStore
	mineGen           *model.MineGenerator
	closureDuration   time.Duration
	workersMu         sync.Mutex
	workers           map[string]*chunkWorker
	workerIdleTimeout time.Duration
	workerQueueSize   int
}

type chunkOperationKind string

const (
	chunkOperationOpen chunkOperationKind = "open"
	chunkOperationFlag chunkOperationKind = "flag"
)

type chunkOperation struct {
	kind    chunkOperationKind
	ctx     context.Context
	userID  int64
	chunkID model.ChunkID
	openReq *req.OpenMineCellRequest
	flagReq *req.FlagMineCellRequest
	result  chan chunkOperationResult
}

type chunkOperationResult struct {
	openResp *res.OpenMineCellResponse
	flagResp *res.FlagMineCellResponse
	err      error
}

type chunkWorker struct {
	chunkID       string
	ops           chan chunkOperation
	activeSenders int
}

func NewChunkService() *ChunkService {
	return &ChunkService{
		mineGen:           model.NewMineGenerator(),
		closureDuration:   defaultChunkClosureDuration,
		workers:           make(map[string]*chunkWorker),
		workerIdleTimeout: defaultChunkWorkerIdleTimeout,
		workerQueueSize:   defaultChunkWorkerQueueSize,
	}
}

func NewChunkServiceWithDeps(seasonReader MineSeasonReader, stateStore MineChunkStateStore, closureDurations ...time.Duration) *ChunkService {
	return &ChunkService{
		seasonReader:      seasonReader,
		stateStore:        stateStore,
		mineGen:           model.NewMineGenerator(),
		closureDuration:   resolveChunkClosureDuration(closureDurations...),
		workers:           make(map[string]*chunkWorker),
		workerIdleTimeout: defaultChunkWorkerIdleTimeout,
		workerQueueSize:   defaultChunkWorkerQueueSize,
	}
}

func resolveChunkClosureDuration(closureDurations ...time.Duration) time.Duration {
	if len(closureDurations) > 0 && closureDurations[0] > 0 {
		return closureDurations[0]
	}
	return defaultChunkClosureDuration
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
	if chunkID.Z == model.ChunkMaxLevel {
		state, err := s.loadChunkState(ctx, season.ID, chunkID.String())
		if err != nil {
			return nil, err
		}
		if err := s.reopenExpiredChunk(ctx, state, time.Now()); err != nil {
			return nil, err
		}
		applyMineStateToSummary(summary, state)
	}
	closedLeafIDs, err := s.stateStore.ListClosedLeafChunkIDs(ctx, season.ID)
	if err != nil {
		return nil, err
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
		if err := s.reopenExpiredChunk(ctx, state, time.Now()); err != nil {
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
	leafStates := make(map[string]*model.MineChunkState)
	if s.hasMineDeps() {
		season, err = s.activeSeason(ctx)
		if err != nil {
			return nil, err
		}
		if level == model.ChunkMaxLevel {
			now := time.Now()
			for y := minY; y <= maxY; y++ {
				for x := minX; x <= maxX; x++ {
					chunkID := model.ChunkID{Region: "cn", Z: level, X: x, Y: y}
					state, err := s.loadChunkState(ctx, season.ID, chunkID.String())
					if err != nil {
						return nil, err
					}
					if err := s.reopenExpiredChunk(ctx, state, now); err != nil {
						return nil, err
					}
					leafStates[chunkID.String()] = state
				}
			}
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
				if level == model.ChunkMaxLevel {
					state := leafStates[summary.ChunkID]
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
	result, err := s.enqueueChunkOperation(ctx, chunkOperation{
		kind:    chunkOperationOpen,
		ctx:     ctx,
		userID:  userID,
		chunkID: chunkID,
		openReq: openReq,
	})
	if err != nil {
		return nil, err
	}
	return result.openResp, result.err
}

func (s *ChunkService) openCellInWorker(ctx context.Context, userID int64, chunkID model.ChunkID, openReq *req.OpenMineCellRequest) (*res.OpenMineCellResponse, error) {
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
	if err := s.reopenExpiredChunk(ctx, state, time.Now()); err != nil {
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
	if _, ok := state.FlaggedBy[index]; ok {
		return nil, ErrCellFlagged
	}

	now := time.Now()
	isMine, err := s.mineGen.IsMine(season, chunkID, openReq.X, openReq.Y)
	if err != nil {
		return nil, err
	}
	adjacentMines := 0
	if isMine {
		if !playerHasOpenedCell(state, userID) {
			return &res.OpenMineCellResponse{
				ChunkID:  chunkID.String(),
				X:        openReq.X,
				Y:        openReq.Y,
				Index:    index,
				Mine:     true,
				Closed:   false,
				Canceled: true,
				Reason:   "first_open_mine_protected",
				Version:  state.Version,
				OpenedAt: now,
			}, nil
		}
		closedUntil := now.Add(s.closureDuration)
		state.Closed = true
		state.ClosedBy = userID
		state.ClosedAt = &now
		state.ClosedUntil = &closedUntil
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
		ClosedAt:      state.ClosedAt,
		ClosedUntil:   state.ClosedUntil,
		AdjacentMines: adjacentMines,
		Version:       state.Version,
		OpenedAt:      now,
		OpenedCells:   openedCellsResponse(state.OpenedCells),
	}, nil
}

func playerHasOpenedCell(state *model.MineChunkState, userID int64) bool {
	if state == nil {
		return false
	}
	for _, cell := range state.OpenedCells {
		if cell.OpenedBy == userID {
			return true
		}
	}
	return false
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
		if _, ok := state.FlaggedBy[index]; ok {
			continue
		}
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
	result, err := s.enqueueChunkOperation(ctx, chunkOperation{
		kind:    chunkOperationFlag,
		ctx:     ctx,
		userID:  userID,
		chunkID: chunkID,
		flagReq: flagReq,
	})
	if err != nil {
		return nil, err
	}
	return result.flagResp, result.err
}

func (s *ChunkService) flagCellInWorker(ctx context.Context, userID int64, chunkID model.ChunkID, flagReq *req.FlagMineCellRequest) (*res.FlagMineCellResponse, error) {
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
	if err := s.reopenExpiredChunk(ctx, state, time.Now()); err != nil {
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

func (s *ChunkService) enqueueChunkOperation(ctx context.Context, op chunkOperation) (chunkOperationResult, error) {
	if err := ctx.Err(); err != nil {
		return chunkOperationResult{}, err
	}
	op.result = make(chan chunkOperationResult, 1)
	worker := s.acquireChunkWorker(op.chunkID.String())
	defer s.releaseChunkWorker(worker)

	select {
	case worker.ops <- op:
	case <-ctx.Done():
		return chunkOperationResult{}, ctx.Err()
	}

	select {
	case result := <-op.result:
		return result, nil
	case <-ctx.Done():
		return chunkOperationResult{}, ctx.Err()
	}
}

func (s *ChunkService) acquireChunkWorker(chunkID string) *chunkWorker {
	s.workersMu.Lock()
	defer s.workersMu.Unlock()
	if s.workers == nil {
		s.workers = make(map[string]*chunkWorker)
	}
	if worker := s.workers[chunkID]; worker != nil {
		worker.activeSenders++
		return worker
	}
	queueSize := s.workerQueueSize
	if queueSize <= 0 {
		queueSize = defaultChunkWorkerQueueSize
	}
	worker := &chunkWorker{
		chunkID: chunkID,
		ops:     make(chan chunkOperation, queueSize),
	}
	worker.activeSenders = 1
	s.workers[chunkID] = worker
	go s.runChunkWorker(worker)
	return worker
}

func (s *ChunkService) releaseChunkWorker(worker *chunkWorker) {
	s.workersMu.Lock()
	defer s.workersMu.Unlock()
	if worker.activeSenders > 0 {
		worker.activeSenders--
	}
}

func (s *ChunkService) runChunkWorker(worker *chunkWorker) {
	idleTimeout := s.workerIdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = defaultChunkWorkerIdleTimeout
	}
	idleTimer := time.NewTimer(idleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case op := <-worker.ops:
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			s.handleChunkOperation(op)
			idleTimer.Reset(idleTimeout)
		case <-idleTimer.C:
			if s.removeIdleChunkWorker(worker) {
				return
			}
			idleTimer.Reset(idleTimeout)
		}
	}
}

func (s *ChunkService) handleChunkOperation(op chunkOperation) {
	if err := op.ctx.Err(); err != nil {
		op.result <- chunkOperationResult{err: err}
		return
	}

	var result chunkOperationResult
	switch op.kind {
	case chunkOperationOpen:
		result.openResp, result.err = s.openCellInWorker(op.ctx, op.userID, op.chunkID, op.openReq)
	case chunkOperationFlag:
		result.flagResp, result.err = s.flagCellInWorker(op.ctx, op.userID, op.chunkID, op.flagReq)
	default:
		result.err = errors.New("unknown chunk operation")
	}
	op.result <- result
}

func (s *ChunkService) removeIdleChunkWorker(worker *chunkWorker) bool {
	s.workersMu.Lock()
	defer s.workersMu.Unlock()
	if s.workers[worker.chunkID] != worker {
		return true
	}
	if worker.activeSenders > 0 || len(worker.ops) > 0 {
		return false
	}
	delete(s.workers, worker.chunkID)
	return true
}

func (s *ChunkService) chunkWorkerCount() int {
	s.workersMu.Lock()
	defer s.workersMu.Unlock()
	return len(s.workers)
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

func (s *ChunkService) reopenExpiredChunk(ctx context.Context, state *model.MineChunkState, now time.Time) error {
	if state == nil || !state.Closed {
		return nil
	}
	changed := false
	if state.ClosedUntil == nil {
		if state.ClosedAt == nil {
			closedAt := now
			state.ClosedAt = &closedAt
		}
		closedUntil := state.ClosedAt.Add(s.closureDuration)
		state.ClosedUntil = &closedUntil
		changed = true
	}
	if now.Before(*state.ClosedUntil) {
		if changed {
			return s.stateStore.SaveState(ctx, state)
		}
		return nil
	}
	state.Closed = false
	state.ClosedBy = 0
	state.ClosedAt = nil
	state.ClosedUntil = nil
	state.Version++
	return s.stateStore.SaveState(ctx, state)
}

func applyMineStateToSummary(summary *res.ChunkSummaryResponse, state *model.MineChunkState) {
	if state == nil {
		summary.State = "normal"
		summary.Closed = false
		summary.ClosedAt = nil
		summary.ClosedUntil = nil
		summary.OpenedCount = 0
		summary.Version = 1
		return
	}
	summary.Closed = state.Closed
	summary.ClosedAt = state.ClosedAt
	summary.ClosedUntil = state.ClosedUntil
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
		summary.Closed = summary.Closed || closedLeafCount == 1
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
	snapshot.ClosedAt = state.ClosedAt
	snapshot.ClosedUntil = state.ClosedUntil
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
