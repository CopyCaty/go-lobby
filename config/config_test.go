package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMineChunkClosureDurationDefault(t *testing.T) {
	path := writeTestConfig(t, `
server:
  addr: ":8080"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.Mine.ChunkClosureDurationValue(); got != DefaultMineChunkClosureDuration {
		t.Fatalf("unexpected default closure duration: got %v want %v", got, DefaultMineChunkClosureDuration)
	}
}

func TestLoadMineChunkClosureDuration(t *testing.T) {
	path := writeTestConfig(t, `
mine:
  chunk_closure_duration: "30s"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.Mine.ChunkClosureDurationValue(); got != 30*time.Second {
		t.Fatalf("unexpected closure duration: got %v want %v", got, 30*time.Second)
	}
}

func TestLoadMineChunkClosureDurationInvalid(t *testing.T) {
	path := writeTestConfig(t, `
mine:
  chunk_closure_duration: "bad-duration"
`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load should reject invalid mine.chunk_closure_duration")
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}
