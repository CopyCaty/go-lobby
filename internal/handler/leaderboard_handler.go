package handler

import (
	"go-lobby/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type LeaderboardHandler struct {
	rankService *service.RankService
}

func NewLeaderboardHandler(rankService *service.RankService) *LeaderboardHandler {
	return &LeaderboardHandler{
		rankService: rankService,
	}
}

func (h *LeaderboardHandler) Top(c *gin.Context) {
	mode := c.Query("mode")
	limitRaw := c.DefaultQuery("limit", "20")
	limit, err := strconv.ParseInt(limitRaw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "limit 参数错误",
		})
		return
	}
	result, err := h.rankService.GetLeaderboard(c.Request.Context(), mode, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    result,
	})
}
