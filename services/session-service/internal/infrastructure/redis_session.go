package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisSessionService struct {
	client *redis.Client
}

type SessionData struct {
	UserID      string    `json:"user_id"`
	AccessToken string    `json:"access_token"`
	RoomID      string    `json:"room_id,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	LastSeen    time.Time `json:"last_seen"`
	IsActive    bool      `json:"is_active"`
}

type UploadSessionData struct {
	UserID      string    `json:"user_id"`
	RoomID      string    `json:"room_id"`
	State       string    `json:"state"` // started, finalizing, completed, revoked
	ChunkCount  int       `json:"chunk_count"`
	StartedAt   time.Time `json:"started_at"`
	LastChunkAt time.Time `json:"last_chunk_at"`
}

const (
	SessionStateActive   = "active"
	SessionStateInactive = "inactive"
	
	UploadStateStarted    = "started"
	UploadStateFinalizing = "finalizing"
	UploadStateCompleted  = "completed"
	UploadStateRevoked    = "revoked"
	
	SessionKeyPattern    = "session:%s"
	UploadKeyPattern     = "upload:%s"
	UserSessionsPattern  = "user_sessions:%s"
)

func NewRedisSessionService(redisClient *redis.Client) *RedisSessionService {
	return &RedisSessionService{
		client: redisClient,
	}
}

func (r *RedisSessionService) CreateSession(sessionID, userID, accessToken, roomID string, ttl time.Duration) error {
	ctx := context.Background()
	
	sessionData := SessionData{
		UserID:      userID,
		AccessToken: accessToken,
		RoomID:      roomID,
		StartedAt:   time.Now(),
		LastSeen:    time.Now(),
		IsActive:    true,
	}
	
	data, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	
	sessionKey := fmt.Sprintf(SessionKeyPattern, sessionID)
	if err := r.client.Set(ctx, sessionKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}
	
	userSessionsKey := fmt.Sprintf(UserSessionsPattern, userID)
	if err := r.client.SAdd(ctx, userSessionsKey, sessionID).Err(); err != nil {
		log.Printf("Warning: failed to add session to user sessions set: %v", err)
	}
	
	r.client.Expire(ctx, userSessionsKey, ttl)
	
	log.Printf("Session created: %s for user: %s, room: %s", sessionID, userID, roomID)
	return nil
}

func (r *RedisSessionService) GetSession(sessionID string) (*SessionData, error) {
	ctx := context.Background()
	
	sessionKey := fmt.Sprintf(SessionKeyPattern, sessionID)
	data, err := r.client.Get(ctx, sessionKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	var sessionData SessionData
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}
	
	return &sessionData, nil
}

func (r *RedisSessionService) ValidateSession(sessionID string) (bool, *SessionData, error) {
	sessionData, err := r.GetSession(sessionID)
	if err != nil {
		return false, nil, err
	}
	
	if !sessionData.IsActive {
		return false, sessionData, fmt.Errorf("session is inactive")
	}
	
	sessionData.LastSeen = time.Now()
	r.UpdateSession(sessionID, sessionData)
	
	return true, sessionData, nil
}

func (r *RedisSessionService) UpdateSession(sessionID string, sessionData *SessionData) error {
	ctx := context.Background()
	
	data, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	
	sessionKey := fmt.Sprintf(SessionKeyPattern, sessionID)
	ttl := r.client.TTL(ctx, sessionKey).Val()
	if ttl > 0 {
		return r.client.Set(ctx, sessionKey, data, ttl).Err()
	}
	
	return r.client.Set(ctx, sessionKey, data, 24*time.Hour).Err()
}

func (r *RedisSessionService) DeleteSession(sessionID string) error {
	ctx := context.Background()
	
	sessionData, err := r.GetSession(sessionID)
	if err == nil && sessionData != nil {
		userSessionsKey := fmt.Sprintf(UserSessionsPattern, sessionData.UserID)
		r.client.SRem(ctx, userSessionsKey, sessionID)
	}
	
	sessionKey := fmt.Sprintf(SessionKeyPattern, sessionID)
	return r.client.Del(ctx, sessionKey).Err()
}

func (r *RedisSessionService) CreateUploadSession(uploadID, userID, roomID string, ttl time.Duration) error {
	ctx := context.Background()
	
	uploadData := UploadSessionData{
		UserID:      userID,
		RoomID:      roomID,
		State:       UploadStateStarted,
		ChunkCount:  0,
		StartedAt:   time.Now(),
		LastChunkAt: time.Now(),
	}
	
	data, err := json.Marshal(uploadData)
	if err != nil {
		return fmt.Errorf("failed to marshal upload session data: %w", err)
	}
	
	uploadKey := fmt.Sprintf(UploadKeyPattern, uploadID)
	if err := r.client.Set(ctx, uploadKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store upload session: %w", err)
	}
	
	log.Printf("Upload session created: %s for user: %s, room: %s", uploadID, userID, roomID)
	return nil
}

func (r *RedisSessionService) GetUploadSession(uploadID string) (*UploadSessionData, error) {
	ctx := context.Background()
	
	uploadKey := fmt.Sprintf(UploadKeyPattern, uploadID)
	data, err := r.client.Get(ctx, uploadKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("upload session not found")
		}
		return nil, fmt.Errorf("failed to get upload session: %w", err)
	}
	
	var uploadData UploadSessionData
	if err := json.Unmarshal([]byte(data), &uploadData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal upload session data: %w", err)
	}
	
	return &uploadData, nil
}

// UpdateUploadSession updates upload session state
func (r *RedisSessionService) UpdateUploadSession(uploadID string, uploadData *UploadSessionData) error {
	ctx := context.Background()
	
	data, err := json.Marshal(uploadData)
	if err != nil {
		return fmt.Errorf("failed to marshal upload session data: %w", err)
	}
	
	uploadKey := fmt.Sprintf(UploadKeyPattern, uploadID)
	ttl := r.client.TTL(ctx, uploadKey).Val()
	if ttl > 0 {
		return r.client.Set(ctx, uploadKey, data, ttl).Err()
	}
	
	return r.client.Set(ctx, uploadKey, data, 2*time.Hour).Err()
}

func (r *RedisSessionService) DeleteUploadSession(uploadID string) error {
	ctx := context.Background()
	uploadKey := fmt.Sprintf(UploadKeyPattern, uploadID)
	return r.client.Del(ctx, uploadKey).Err()
}

// RevokeUserSessions revokes all sessions for a user
func (r *RedisSessionService) RevokeUserSessions(userID string) error {
	ctx := context.Background()
	
	userSessionsKey := fmt.Sprintf(UserSessionsPattern, userID)
	sessionIDs, err := r.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}
	
	// Delete all sessions
	for _, sessionID := range sessionIDs {
		if err := r.DeleteSession(sessionID); err != nil {
			log.Printf("Warning: failed to delete session %s: %v", sessionID, err)
		}
	}
	
	return r.client.Del(ctx, userSessionsKey).Err()
}



