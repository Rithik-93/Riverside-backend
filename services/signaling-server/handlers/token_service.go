package handlers

import (
	"os"
)

type TokenClaims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
}

type TokenService struct {
	accessTokenSecret string
}

func NewTokenService() *TokenService {
	return &TokenService{
		accessTokenSecret: os.Getenv("JWT_ACCESS_SECRET"),
	}
}
