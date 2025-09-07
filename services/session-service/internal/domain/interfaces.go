package domain

import (
	"github.com/lakeside/services/session-service/pkg/types"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(user *User) error
	GetByID(id string) (*User, error)
	GetByEmail(email string) (*User, error)
	GetByUsername(username string) (*User, error)
	Update(user *User) error
	Delete(id string) error
}

// SessionRepository defines the interface for session data operations
type SessionRepository interface {
	Create(session *Session) error
	GetByID(id string) (*Session, error)
	GetByAccessToken(accessToken string) (*Session, error)
	GetByRefreshToken(refreshToken string) (*Session, error)
	Update(session *Session) error
	Delete(id string) error
	DeleteByRefreshToken(refreshToken string) error
	DeleteByUserID(userID string) error
}

// TokenService defines the interface for token operations
type TokenService interface {
	GenerateTokenPair(userID, email, username string) (string, string, error)
	ValidateAccessToken(token string) (*types.TokenClaims, error)
	ValidateRefreshToken(token string) (*types.TokenClaims, error)
}

// OAuthService defines the interface for OAuth operations
type OAuthService interface {
	GetAuthURL(provider string) (string, string, error)
	ExchangeCode(provider, code, state string) (*types.OAuthUser, error)
	ValidateState(state string) bool
}
