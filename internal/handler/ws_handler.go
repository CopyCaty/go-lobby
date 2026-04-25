package handler

import (
	"fmt"
	"go-lobby/internal/middleware"
	"go-lobby/internal/service"
	"go-lobby/internal/ws"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSHandler struct {
	rs *service.RoomService
	rh *ws.RoomHub
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewWSHandler(rs *service.RoomService, rh *ws.RoomHub) *WSHandler {
	return &WSHandler{
		rs: rs,
		rh: rh,
	}
}

func (h *WSHandler) JoinRoom(c *gin.Context) {
	roomID := c.Param("id")
	rawUserID, exist := c.Get(middleware.CtxUserIDKey)
	if !exist {
		c.JSON(401, gin.H{
			"code":    401,
			"message": "未授权",
		})
		return
	}
	userID := rawUserID.(int64)
	fmt.Println("roomID: ", roomID)
	fmt.Println("userID: ", userID)
	if !h.rs.CheckUserInRoom(roomID, userID) {
		fmt.Printf("User %d is not in room %s\n", userID, roomID)
		c.JSON(403, gin.H{
			"code":    403,
			"message": "用户不在房间内",
		})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "WebSocket升级失败",
		})
		return
	}
	h.rh.JoinRoom(roomID, userID, conn)
	defer func() {
		conn.Close()
		h.rh.LeaveRoom(roomID, userID)
	}()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		h.rh.BroadcastToRoom(roomID, data)
	}
}
