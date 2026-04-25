package handler

import (
	"encoding/json"
	"go-lobby/internal/middleware"
	"go-lobby/internal/service"
	"go-lobby/internal/ws"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	service *service.RoomService
	hub     *ws.RoomHub
}

func NewRoomHandler(service *service.RoomService, hub *ws.RoomHub) *RoomHandler {
	return &RoomHandler{service: service, hub: hub}
}

func (h *RoomHandler) GetRoom(c *gin.Context) {
	roomID := c.Param("id")
	rawUserID, exists := c.Get(middleware.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未授权",
		})
		return
	}
	userID := rawUserID.(int64)
	room, err := h.service.GetRoom(roomID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "房间不存在",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    room,
	})
}

func (h *RoomHandler) Ready(c *gin.Context) {
	roomID := c.Param("id")
	rawUserID, exists := c.Get(middleware.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未授权",
		})
		return
	}
	userID := rawUserID.(int64)
	err := h.service.ReadyPlayer(roomID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "房间或玩家不存在",
		})
		return
	}
	room, err := h.service.GetRoom(roomID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取房间信息失败",
		})
		return
	}
	payload := gin.H{
		"type":    "player_ready",
		"room_id": roomID,
		"data":    room,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "序列化数据失败",
		})
		return
	}
	h.hub.BroadcastToRoom(roomID, data)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}
