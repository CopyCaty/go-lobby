package model

const (
	ChunkSize       = 128
	ChunkCellCount  = ChunkSize * ChunkSize
	ChunkBitmapSize = ChunkCellCount / 8
)

type ChunkID struct {
	Region string
	Z      int
	X      int
	Y      int
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
