package service

import (
	"context"
	"errors"
	"go-lobby/internal/model"
	"testing"
)

func TestChunkServiceGetChunkSummary(t *testing.T) {
	svc := NewChunkService()

	summary, err := svc.GetChunkSummary(context.Background(), "cn:6:10:20")
	if err != nil {
		t.Fatalf("GetChunkSummary returned error: %v", err)
	}
	if summary.ChunkID != "cn:6:10:20" {
		t.Fatalf("unexpected chunk ID: %s", summary.ChunkID)
	}
	if summary.Level != 6 || summary.Z != 6 {
		t.Fatalf("unexpected level: %d/%d", summary.Level, summary.Z)
	}
	if summary.Width != 128 || summary.Height != 128 {
		t.Fatalf("unexpected chunk size: %dx%d", summary.Width, summary.Height)
	}
	if summary.OpenedCount != 256 {
		t.Fatalf("unexpected opened count: %d", summary.OpenedCount)
	}
	if summary.Bounds.MinX != 0.15625 || summary.Bounds.MinY != 0.3125 {
		t.Fatalf("unexpected bounds: %+v", summary.Bounds)
	}
}

func TestChunkServiceGetChunkSnapshot(t *testing.T) {
	svc := NewChunkService()

	snapshot, err := svc.GetChunkSnapshot(context.Background(), "cn:6:10:20")
	if err != nil {
		t.Fatalf("GetChunkSnapshot returned error: %v", err)
	}
	if len(snapshot.OpenedCells) != 256 {
		t.Fatalf("unexpected opened cell count: %d", len(snapshot.OpenedCells))
	}
	first := snapshot.OpenedCells[0]
	if first.X != 64 || first.Y != 42 || first.Index != 5440 {
		t.Fatalf("unexpected first opened cell: %+v", first)
	}
	if first.OpenedBy.UserID == 0 || first.OpenedBy.Nickname == "" {
		t.Fatalf("opened_by should be populated: %+v", first.OpenedBy)
	}
}

func TestChunkServiceListChunks(t *testing.T) {
	svc := NewChunkService()

	result, err := svc.ListChunks(context.Background(), 6, modelChunkBounds(0.15, 0.31, 0.18, 0.34))
	if err != nil {
		t.Fatalf("ListChunks returned error: %v", err)
	}
	if result.Level != 6 || len(result.Chunks) != 9 {
		t.Fatalf("unexpected list result: level=%d len=%d", result.Level, len(result.Chunks))
	}
	if result.Chunks[0].ChunkID != "cn:6:9:19" {
		t.Fatalf("unexpected first chunk: %s", result.Chunks[0].ChunkID)
	}
}

func TestChunkServiceErrors(t *testing.T) {
	svc := NewChunkService()

	if _, err := svc.GetChunkSummary(context.Background(), "bad-id"); !errors.Is(err, ErrInvalidChunkID) {
		t.Fatalf("expected ErrInvalidChunkID, got %v", err)
	}
	if _, err := svc.GetChunkSummary(context.Background(), "cn:0:1:0"); !errors.Is(err, ErrChunkNotFound) {
		t.Fatalf("expected ErrChunkNotFound, got %v", err)
	}
	if _, err := svc.ListChunks(context.Background(), 7, modelChunkBounds(0, 0, 1, 1)); !errors.Is(err, ErrInvalidChunkID) {
		t.Fatalf("expected ErrInvalidChunkID, got %v", err)
	}
}

func modelChunkBounds(minX, minY, maxX, maxY float64) model.ChunkBounds {
	return model.ChunkBounds{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
}
