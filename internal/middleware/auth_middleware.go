package middleware

import (
	"net/http"
	"strings"

	"go-lobby/internal/auth"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserIDKey   = "user_id"
	CtxUserNameKey = "user_name"
)

func AuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录",
			})
			c.Abort()
			return
		}

		claims, err := jwtManager.ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Token 无效或已过期",
			})
			c.Abort()
			return
		}

		c.Set(CtxUserIDKey, claims.UserID)
		c.Set(CtxUserNameKey, claims.UserName)
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if strings.HasPrefix(authHeader, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
	}
	if token := strings.TrimSpace(c.Query("token")); token != "" {
		return token
	}
	token, err := c.Cookie("access_token")
	if err != nil {
		return ""
	}
	return token
}
