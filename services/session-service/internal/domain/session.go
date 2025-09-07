package domain

import (
	"time"
)

type Session struct {
	ID           string    `gorm:"primaryKey"`
	UserID       string    `gorm:"not null"`
	AccessToken  string    `gorm:"uniqueIndex;not null"`
	RefreshToken string    `gorm:"uniqueIndex;not null"`
	ExpiresAt    time.Time `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewSession(userID, accessToken, refreshToken string, expiresIn time.Duration) *Session {
	now := time.Now()
	return &Session{
		ID:           generateID(),
		UserID:       userID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(expiresIn),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) Refresh(accessToken, refreshToken string, expiresIn time.Duration) {
	s.AccessToken = accessToken
	s.RefreshToken = refreshToken
	s.ExpiresAt = time.Now().Add(expiresIn)
	s.UpdatedAt = time.Now()
}
