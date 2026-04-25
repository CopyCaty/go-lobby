package handler

import (
	"go-lobby/internal/dto/req"
	"go-lobby/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type MatchHandler struct {
	matchService *service.MatchService
}

func NewMatchHandler(matchService *service.MatchService) *MatchHandler {
	return &MatchHandler{
		matchService: matchService,
	}
}

func (h *MatchHandler) SetMatchResult(c *gin.Context) {
	var req req.SetMatchResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
		})
		return
	}
	err := h.matchService.SetMatchResult(c.Request.Context(), req.MatchID, req.WinTeamNo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

func (h *MatchHandler) GetMatchInfo(c *gin.Context) {
	rawMatchID := c.Param("id")

	matchID, err := strconv.ParseInt(rawMatchID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid match ID",
		})
		return
	}
	matchInfo, err := h.matchService.GetMatchInfo(c.Request.Context(), matchID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    matchInfo,
	})
}
