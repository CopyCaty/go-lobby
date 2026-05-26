package model

import "testing"

func TestParseChunkID(t *testing.T) {
	chunkID, err := ParseChunkID("cn:0:0:0")
	if err != nil {
		t.Fatalf("ParseChunkID returned error: %v", err)
	}
	if chunkID.Region != "cn" || chunkID.Z != 0 || chunkID.X != 0 || chunkID.Y != 0 {
		t.Fatalf("unexpected chunk ID: %+v", chunkID)
	}
	if chunkID.String() != "cn:0:0:0" {
		t.Fatalf("unexpected chunk ID string: %s", chunkID.String())
	}
}

func TestParseChunkIDInvalid(t *testing.T) {
	cases := []string{
		"bad-id",
		":0:0:0",
		"cn:a:0:0",
		"cn:0:-1:0",
		"cn:0:0",
	}

	for _, tc := range cases {
		if _, err := ParseChunkID(tc); err == nil {
			t.Fatalf("ParseChunkID(%q) expected error", tc)
		}
	}
}

func TestChunkCellIndex(t *testing.T) {
	index, err := ChunkCellIndex(0, 0)
	if err != nil {
		t.Fatalf("ChunkCellIndex returned error: %v", err)
	}
	if index != 0 {
		t.Fatalf("unexpected index for (0,0): %d", index)
	}

	index, err = ChunkCellIndex(127, 127)
	if err != nil {
		t.Fatalf("ChunkCellIndex returned error: %v", err)
	}
	if index != 16383 {
		t.Fatalf("unexpected index for (127,127): %d", index)
	}
}

func TestChunkCellIndexOutOfRange(t *testing.T) {
	if _, err := ChunkCellIndex(128, 0); err == nil {
		t.Fatal("expected out of range error")
	}
}

func TestChunkBoundsFor(t *testing.T) {
	bounds, err := ChunkBoundsFor(6, 10, 20)
	if err != nil {
		t.Fatalf("ChunkBoundsFor returned error: %v", err)
	}
	if bounds.MinX != 0.15625 || bounds.MaxX != 0.171875 {
		t.Fatalf("unexpected x bounds: %+v", bounds)
	}
	if bounds.MinY != 0.3125 || bounds.MaxY != 0.328125 {
		t.Fatalf("unexpected y bounds: %+v", bounds)
	}
}

func TestVisibleChunkRange(t *testing.T) {
	minX, minY, maxX, maxY, err := VisibleChunkRange(6, ChunkBounds{
		MinX: 0.15,
		MinY: 0.31,
		MaxX: 0.18,
		MaxY: 0.34,
	})
	if err != nil {
		t.Fatalf("VisibleChunkRange returned error: %v", err)
	}
	if minX != 9 || maxX != 11 || minY != 19 || maxY != 21 {
		t.Fatalf("unexpected range: %d,%d to %d,%d", minX, minY, maxX, maxY)
	}
}

func TestVisibleChunkRangeInvalid(t *testing.T) {
	if _, _, _, _, err := VisibleChunkRange(7, ChunkBounds{MinX: 0, MinY: 0, MaxX: 1, MaxY: 1}); err == nil {
		t.Fatal("expected invalid level error")
	}
	if _, _, _, _, err := VisibleChunkRange(1, ChunkBounds{MinX: 0.5, MinY: 0, MaxX: 0.4, MaxY: 1}); err == nil {
		t.Fatal("expected invalid bounds error")
	}
}
