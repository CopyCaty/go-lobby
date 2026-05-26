package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go-lobby/internal/dto/req"
	"go-lobby/internal/dto/res"
	"go-lobby/internal/middleware"
	"go-lobby/internal/model"
	"go-lobby/internal/service"
	"go-lobby/internal/ws"

	"github.com/gin-gonic/gin"
)

type ChunkHandler struct {
	chunkService *service.ChunkService
	mapHub       *ws.MapHub
}

func NewChunkHandler(chunkService *service.ChunkService, mapHub ...*ws.MapHub) *ChunkHandler {
	var hub *ws.MapHub
	if len(mapHub) > 0 {
		hub = mapHub[0]
	}
	return &ChunkHandler{
		chunkService: chunkService,
		mapHub:       hub,
	}
}

func (h *ChunkHandler) ListChunks(c *gin.Context) {
	level, err := strconv.Atoi(c.Query("level"))
	if err != nil {
		writeChunkError(c, service.ErrInvalidChunkID)
		return
	}
	bbox, err := parseChunkBBox(c)
	if err != nil {
		writeChunkError(c, service.ErrInvalidChunkID)
		return
	}

	chunks, err := h.chunkService.ListChunks(c.Request.Context(), level, bbox)
	if err != nil {
		writeChunkError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    chunks,
	})
}

func (h *ChunkHandler) GetChunk(c *gin.Context) {
	chunk, err := h.chunkService.GetChunkSummary(c.Request.Context(), c.Param("chunk_id"))
	if err != nil {
		writeChunkError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    chunk,
	})
}

func (h *ChunkHandler) GetSnapshot(c *gin.Context) {
	snapshot, err := h.chunkService.GetChunkSnapshot(c.Request.Context(), c.Param("chunk_id"))
	if err != nil {
		writeChunkError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    snapshot,
	})
}

func (h *ChunkHandler) OpenCell(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}
	var openReq req.OpenMineCellRequest
	if err := c.ShouldBindJSON(&openReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
		})
		return
	}
	result, err := h.chunkService.OpenCell(c.Request.Context(), userID, c.Param("chunk_id"), &openReq)
	if err != nil {
		writeChunkError(c, err)
		return
	}
	h.broadcastOpenCell(result)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    result,
	})
}

func (h *ChunkHandler) FlagCell(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}
	var flagReq req.FlagMineCellRequest
	if err := c.ShouldBindJSON(&flagReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
		})
		return
	}
	result, err := h.chunkService.FlagCell(c.Request.Context(), userID, c.Param("chunk_id"), &flagReq)
	if err != nil {
		writeChunkError(c, err)
		return
	}
	h.broadcastFlagCell(result)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    result,
	})
}

func parseChunkBBox(c *gin.Context) (model.ChunkBounds, error) {
	minX, err := strconv.ParseFloat(c.Query("min_x"), 64)
	if err != nil {
		return model.ChunkBounds{}, err
	}
	minY, err := strconv.ParseFloat(c.Query("min_y"), 64)
	if err != nil {
		return model.ChunkBounds{}, err
	}
	maxX, err := strconv.ParseFloat(c.Query("max_x"), 64)
	if err != nil {
		return model.ChunkBounds{}, err
	}
	maxY, err := strconv.ParseFloat(c.Query("max_y"), 64)
	if err != nil {
		return model.ChunkBounds{}, err
	}
	return model.ChunkBounds{
		MinX: minX,
		MinY: minY,
		MaxX: maxX,
		MaxY: maxY,
	}, nil
}

func currentUserID(c *gin.Context) (int64, bool) {
	rawUserID, exist := c.Get(middleware.CtxUserIDKey)
	if !exist {
		return 0, false
	}
	userID, ok := rawUserID.(int64)
	return userID, ok
}

func (h *ChunkHandler) broadcastOpenCell(result *res.OpenMineCellResponse) {
	if h.mapHub == nil || result == nil {
		return
	}
	eventType := "mine.cell_opened"
	if result.Closed {
		eventType = "mine.chunk_closed"
	}
	h.mapHub.BroadcastChunkEvent(ws.ChunkEvent{
		Type:    eventType,
		ChunkID: result.ChunkID,
		Version: result.Version,
		Data:    result,
	})
}

func (h *ChunkHandler) broadcastFlagCell(result *res.FlagMineCellResponse) {
	if h.mapHub == nil || result == nil {
		return
	}
	h.mapHub.BroadcastChunkEvent(ws.ChunkEvent{
		Type:    "mine.cell_flagged",
		ChunkID: result.ChunkID,
		Version: result.Version,
		Data:    result,
	})
}

func writeChunkError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrInvalidChunkID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "chunk_id 参数错误",
		})
		return
	}
	if errors.Is(err, service.ErrChunkNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "chunk 不存在",
		})
		return
	}
	if errors.Is(err, service.ErrNoActiveSeason) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "当前没有可用赛季",
		})
		return
	}
	if errors.Is(err, service.ErrChunkClosed) {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "chunk 已封闭",
		})
		return
	}
	if errors.Is(err, service.ErrCellAlreadyOpen) {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": "格子已打开",
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    500,
		"message": err.Error(),
	})
}
