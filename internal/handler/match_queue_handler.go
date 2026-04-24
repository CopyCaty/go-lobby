package handler

import (
	"go-lobby/internal/dto/req"
	"go-lobby/internal/middleware"
	"go-lobby/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MatchQueueHandler struct {
	matchQueueService *service.MatchQueueService
}

func NewMatchQueueHandler(matchQueueService *service.MatchQueueService) *MatchQueueHandler {
	return &MatchQueueHandler{
		matchQueueService: matchQueueService,
	}
}

func (h *MatchQueueHandler) Join(c *gin.Context) {
	var req req.JoinMatchQueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
		})
		return
	}
	rawUserID, exist := c.Get(middleware.CtxUserIDKey)
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}
	userID, ok := rawUserID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户上下文类型错误",
		})
		return
	}
	res, err := h.matchQueueService.Join(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "加入匹配失败",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "加入匹配成功",
		"data":    res,
	})
}

func (h *MatchQueueHandler) Status(c *gin.Context) {
	rawUserID, exist := c.Get(middleware.CtxUserIDKey)
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}
	userID, ok := rawUserID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户上下文类型错误",
		})
		return
	}
	res, err := h.matchQueueService.Status(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "取消匹配失败",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": res.Status,
		"data":    res,
	})
}

func (h *MatchQueueHandler) Cancel(c *gin.Context) {
	rawUserID, exist := c.Get(middleware.CtxUserIDKey)
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未登录",
		})
		return
	}
	userID, ok := rawUserID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户上下文类型错误",
		})
		return
	}
	res, err := h.matchQueueService.Cancel(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "取消匹配失败",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "取消匹配成功",
		"data":    res,
	})
}
