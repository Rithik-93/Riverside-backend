package repository

import (
	"github.com/lakeside/services/session-service/internal/domain"
	"gorm.io/gorm"
)

type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(session *domain.Session) error {
	return r.db.Create(session).Error
}

// GetByID retrieves a session by ID
func (r *SessionRepository) GetByID(id string) (*domain.Session, error) {
	var session domain.Session
	err := r.db.Where("id = ?", id).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetByAccessToken retrieves a session by access token
func (r *SessionRepository) GetByAccessToken(accessToken string) (*domain.Session, error) {
	var session domain.Session
	err := r.db.Where("access_token = ?", accessToken).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetByRefreshToken retrieves a session by refresh token
func (r *SessionRepository) GetByRefreshToken(refreshToken string) (*domain.Session, error) {
	var session domain.Session
	err := r.db.Where("refresh_token = ?", refreshToken).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// Update updates an existing session
func (r *SessionRepository) Update(session *domain.Session) error {
	return r.db.Save(session).Error
}

// Delete deletes a session by ID
func (r *SessionRepository) Delete(id string) error {
	return r.db.Delete(&domain.Session{}, "id = ?", id).Error
}

// DeleteByRefreshToken deletes a session by refresh token
func (r *SessionRepository) DeleteByRefreshToken(refreshToken string) error {
	return r.db.Delete(&domain.Session{}, "refresh_token = ?", refreshToken).Error
}

// DeleteByUserID deletes all sessions for a user
func (r *SessionRepository) DeleteByUserID(userID string) error {
	return r.db.Delete(&domain.Session{}, "user_id = ?", userID).Error
}
