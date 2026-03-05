package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/chowyu12/go-ai-agent/internal/model"
)

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateJWT(secret []byte, expireHours int, u *model.User) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     string(u.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(expireHours) * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func parseJWT(secret []byte, tokenStr string) (*Identity, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(_ *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid jwt token")
	}
	return &Identity{
		Kind:     KindJWT,
		UserID:   claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}, nil
}
