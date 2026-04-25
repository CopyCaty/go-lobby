package handler

import (
	"fmt"
	"go-lobby/internal/middleware"
	"go-lobby/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	service *service.RoomService
}

func NewRoomHandler(service *service.RoomService) *RoomHandler {
	return &RoomHandler{service: service}
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
	fmt.Println("Ready endpoint called")
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
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}
