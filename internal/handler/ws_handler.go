package handler

import (
	"fmt"
	"net/http"

	"go-lobby/internal/middleware"
	"go-lobby/internal/service"
	"go-lobby/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSHandler struct {
	rs *service.RoomService
	rh *ws.RoomHub
}

type MapWSHandler struct {
	mh *ws.MapHub
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

func NewMapWSHandler(mh *ws.MapHub) *MapWSHandler {
	return &MapWSHandler{
		mh: mh,
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
	client := ws.NewClient(h.rh, roomID, userID, conn)
	h.rh.JoinRoom(client)

	go client.WritePump()
	client.ReadPump()
}

func (h *MapWSHandler) JoinMap(c *gin.Context) {
	rawUserID, exist := c.Get(middleware.CtxUserIDKey)
	if !exist {
		c.JSON(401, gin.H{
			"code":    401,
			"message": "未授权",
		})
		return
	}
	userID := rawUserID.(int64)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "WebSocket升级失败",
		})
		return
	}
	client := ws.NewMapClient(h.mh, userID, conn)
	h.mh.Join(client)

	go client.WritePump()
	client.ReadPump()
}
