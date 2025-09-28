package types

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Session struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	AccessToken  string    `json:"access_token" db:"access_token"`
	RefreshToken string    `json:"refresh_token" db:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type TokenClaims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

func (t *TokenClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	if t.Exp == 0 {
		return nil, nil
	}
	exp := jwt.NewNumericDate(time.Unix(t.Exp, 0))
	return exp, nil
}

func (t *TokenClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

func (t *TokenClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	if t.Iat == 0 {
		return nil, nil
	}
	iat := jwt.NewNumericDate(time.Unix(t.Iat, 0))
	return iat, nil
}

func (t *TokenClaims) GetIssuer() (string, error) {
	return "", nil
}

func (t *TokenClaims) GetSubject() (string, error) {
	return "", nil
}

func (t *TokenClaims) GetAudience() (jwt.ClaimStrings, error) {
	return nil, nil
}


type SessionResponse struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}
