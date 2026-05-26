package handler

import (
	"encoding/json"
	"go-lobby/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestChunkHandlerGetChunk(t *testing.T) {
	router := newChunkTestRouter()

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/map/chunks/cn:6:10:20", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := body["data"].(map[string]any)
	if data["chunk_id"] != "cn:6:10:20" {
		t.Fatalf("unexpected chunk_id: %v", data["chunk_id"])
	}
	if data["opened_count"].(float64) != 256 {
		t.Fatalf("unexpected opened_count: %v", data["opened_count"])
	}
}

func TestChunkHandlerListChunks(t *testing.T) {
	router := newChunkTestRouter()

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/map/chunks?level=6&min_x=0.15&min_y=0.31&max_x=0.18&max_y=0.34", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := body["data"].(map[string]any)
	chunks := data["chunks"].([]any)
	if len(chunks) != 9 {
		t.Fatalf("unexpected chunk count: %d", len(chunks))
	}
}

func TestChunkHandlerGetSnapshot(t *testing.T) {
	router := newChunkTestRouter()

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/map/chunks/cn:0:0:0/snapshot", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := body["data"].(map[string]any)
	if _, ok := data["cells"]; ok {
		t.Fatal("snapshot should not return full cells")
	}
	openedCells := data["opened_cells"].([]any)
	if len(openedCells) != 256 {
		t.Fatalf("unexpected opened cell count: %d", len(openedCells))
	}
}

func TestChunkHandlerErrors(t *testing.T) {
	router := newChunkTestRouter()

	cases := []struct {
		path string
		want int
	}{
		{path: "/api/v1/map/chunks/bad-id", want: http.StatusBadRequest},
		{path: "/api/v1/map/chunks/cn:0:1:0", want: http.StatusNotFound},
		{path: "/api/v1/map/chunks?level=bad&min_x=0&min_y=0&max_x=1&max_y=1", want: http.StatusBadRequest},
		{path: "/api/v1/map/chunks?level=7&min_x=0&min_y=0&max_x=1&max_y=1", want: http.StatusBadRequest},
	}

	for _, tc := range cases {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		router.ServeHTTP(resp, req)
		if resp.Code != tc.want {
			t.Fatalf("%s unexpected status code: got %d want %d", tc.path, resp.Code, tc.want)
		}
	}
}

func newChunkTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	chunkHandler := NewChunkHandler(service.NewChunkService())
	router.GET("/api/v1/map/chunks", chunkHandler.ListChunks)
	router.GET("/api/v1/map/chunks/:chunk_id", chunkHandler.GetChunk)
	router.GET("/api/v1/map/chunks/:chunk_id/snapshot", chunkHandler.GetSnapshot)
	return router
}
