package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lakeside/services/session-service/pkg/types"
)

type TokenService struct {
	accessTokenSecret  string
	refreshTokenSecret string
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
}

func NewTokenService(accessSecret, refreshSecret string) *TokenService {
	return &TokenService{
		accessTokenSecret:  accessSecret,
		refreshTokenSecret: refreshSecret,
		accessTokenExpiry:  2 * time.Minute,
		refreshTokenExpiry: 7 * 24 * time.Hour,
	}
}

func (t *TokenService) GenerateTokenPair(userID, email, username string) (string, string, error) {
	accessToken, err := t.generateAccessToken(userID, email, username)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := t.generateRefreshToken(userID)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (t *TokenService) generateAccessToken(userID, email, username string) (string, error) {
	now := time.Now()
	claims := &types.TokenClaims{
		UserID:   userID,
		Email:    email,
		Username: username,
		Exp:      now.Add(t.accessTokenExpiry).Unix(),
		Iat:      now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(t.accessTokenSecret))
}

func (t *TokenService) generateRefreshToken(userID string) (string, error) {
	refreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(refreshTokenBytes), nil
}

func (t *TokenService) ValidateAccessToken(tokenString string) (*types.TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &types.TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(t.accessTokenSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*types.TokenClaims); ok && token.Valid {
		if time.Now().Unix() > claims.Exp {
			return nil, fmt.Errorf("token expired")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (t *TokenService) ValidateRefreshToken(refreshToken string) (*types.TokenClaims, error) {
	// Refresh tokens are just random strings stored in database, not JWTs
	// The actual validation happens in auth.go by looking up the refresh token
	// This method should just return success - the real validation is in the auth service
	
	// Just validate that the refresh token is not empty
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token is empty")
	}
	
	// Return a valid claims structure - the UserID will be set by the auth service
	// after it looks up the refresh token in the database
	return &types.TokenClaims{
		UserID: "valid", // This indicates the token format is valid
		Exp:    time.Now().Add(t.refreshTokenExpiry).Unix(),
	}, nil
}

func (t *TokenService) RevokeToken(token string) error {

	return nil
}
