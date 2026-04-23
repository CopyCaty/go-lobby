package auth

import (
	"errors"
	"go-lobby/internal/model"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	UserID   int64  `json:"user_id"`
	UserName string `json:"user_name"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret         []byte
	expireDuration time.Duration
}

func NewJWTManager(secret string, expireDuration time.Duration) *JWTManager {
	return &JWTManager{
		secret:         []byte(secret),
		expireDuration: expireDuration,
	}
}

func (m *JWTManager) GenerateToken(user *model.User) (string, int64, error) {
	now := time.Now()
	expiredAt := now.Add(m.expireDuration)
	claims := UserClaims{
		UserID:   user.ID,
		UserName: user.UserName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiredAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", 0, err
	}
	return tokenString, int64(m.expireDuration.Seconds()), nil
}

func (m *JWTManager) ParseToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&UserClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("不支持的签名算法")
			}
			return m.secret, nil
		},
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, errors.New("无效Token")
	}
	return claims, nil
}
