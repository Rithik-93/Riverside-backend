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
		accessTokenExpiry:  15 * time.Minute,   // 15 minutes
		refreshTokenExpiry: 7 * 24 * time.Hour, // 7 days
	}
}

func (t *TokenService) GenerateTokenPair(userID, email, username string) (string, string, error) {
	// Generate access token
	accessToken, err := t.generateAccessToken(userID, email, username)
	if err != nil {
		return "", "", err
	}

	// Generate refresh token
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
	// Generate a random refresh token
	refreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(refreshTokenBytes), nil
}

// ValidateAccessToken validates an access token
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

	//REMIDER!!!
	// For refresh tokens, we'll use a simple validation
	// In a production system, you might want to store refresh tokens in a database
	// and validate them against stored values

	// For now, we'll return a basic claim structure
	// This should be enhanced with proper refresh token validation
	return &types.TokenClaims{
		UserID: "", // This should be extracted from the stored refresh token
		Exp:    time.Now().Add(t.refreshTokenExpiry).Unix(),
	}, nil
}

func (t *TokenService) RevokeToken(token string) error {

	//REMIDER!!!
	// In a production system, you would add the token to a blacklist
	// or delete it from the database
	return nil
}
