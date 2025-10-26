package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
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
	PodcastID   string    `json:"podcast_id,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	LastSeen    time.Time `json:"last_seen"`
	IsActive    bool      `json:"is_active"`
	IsHost      bool      `json:"is_host"`
}

type UploadSessionData struct {
	UserID      string    `json:"user_id"`
	RoomID      string    `json:"room_id"`
	State       string    `json:"state"`
	ChunkCount  int       `json:"chunk_count"`
	StartedAt   time.Time `json:"started_at"`
	LastChunkAt time.Time `json:"last_chunk_at"`
}

type RecordingSessionData struct {
	RecordingID    string            `json:"recording_id"`
	PodcastID      string            `json:"podcast_id"`
	HostUserID     string            `json:"host_user_id"`
	RoomID         string            `json:"room_id"`
	Participants   []string          `json:"participants"`
	State          string            `json:"state"`
	StartedAt      time.Time         `json:"started_at"`
	EndedAt        *time.Time        `json:"ended_at,omitempty"`
	Duration       int64             `json:"duration"`
	TotalChunks    int               `json:"total_chunks"`
	ParticipantChunks map[string]int `json:"participant_chunks"`
	IsActive       bool              `json:"is_active"`
}

const (
	SessionStateActive   = "active"
	SessionStateInactive = "inactive"
	
	UploadStateStarted    = "started"
	UploadStateFinalizing = "finalizing"
	UploadStateCompleted  = "completed"
	UploadStateRevoked    = "revoked"
	
	RecordingStateStarted    = "started"
	RecordingStateFinalizing = "finalizing"
	RecordingStateCompleted  = "completed"
	RecordingStateRevoked    = "revoked"
	
	SessionKeyPattern         = "session:%s"
	UploadKeyPattern          = "upload:%s"
	UserSessionsPattern       = "user_sessions:%s"
	RecordingKeyPattern       = "recording:%s"
	PodcastRecordingsPattern  = "podcast_recordings:%s"
	UserRecordingsPattern     = "user_recordings:%s"
)

func NewRedisSessionService(redisClient *redis.Client) *RedisSessionService {
	return &RedisSessionService{
		client: redisClient,
	}
}

func (r *RedisSessionService) CreateSession(sessionID, userID, accessToken, roomID, podcastID string, isHost bool, ttl time.Duration) error {
	ctx := context.Background()
	
	sessionData := SessionData{
		UserID:      userID,
		AccessToken: accessToken,
		RoomID:      roomID,
		PodcastID:   podcastID,
		StartedAt:   time.Now(),
		LastSeen:    time.Now(),
		IsActive:    true,
		IsHost:      isHost,
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
    
	var ErrSessionNotFound = errors.New("session not found")

	if err != nil {
        if err == redis.Nil {
            return nil, ErrSessionNotFound
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

func (r *RedisSessionService) CreateRecordingSession(recordingID, podcastID, hostUserID, roomID string, participants []string, ttl time.Duration) error {
	ctx := context.Background()
	
	recordingData := RecordingSessionData{
		RecordingID:       recordingID,
		PodcastID:         podcastID,
		HostUserID:        hostUserID,
		RoomID:            roomID,
		Participants:      participants,
		State:             RecordingStateStarted,
		StartedAt:         time.Now(),
		Duration:          0,
		TotalChunks:       0,
		ParticipantChunks: make(map[string]int),
		IsActive:          true,
	}
	
	data, err := json.Marshal(recordingData)
	if err != nil {
		return fmt.Errorf("failed to marshal recording session data: %w", err)
	}
	
	recordingKey := fmt.Sprintf(RecordingKeyPattern, recordingID)
	if err := r.client.Set(ctx, recordingKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store recording session: %w", err)
	}
	
	podcastRecordingsKey := fmt.Sprintf(PodcastRecordingsPattern, podcastID)
	if err := r.client.SAdd(ctx, podcastRecordingsKey, recordingID).Err(); err != nil {
		log.Printf("Warning: failed to add recording to podcast recordings set: %v", err)
	}
	r.client.Expire(ctx, podcastRecordingsKey, ttl)
	
	userRecordingsKey := fmt.Sprintf(UserRecordingsPattern, hostUserID)
	if err := r.client.SAdd(ctx, userRecordingsKey, recordingID).Err(); err != nil {
		log.Printf("Warning: failed to add recording to user recordings set: %v", err)
	}
	r.client.Expire(ctx, userRecordingsKey, ttl)
	
	log.Printf("Recording session created: %s for host: %s, podcast: %s", recordingID, hostUserID, podcastID)
	return nil
}

func (r *RedisSessionService) GetRecordingSession(recordingID string) (*RecordingSessionData, error) {
	ctx := context.Background()
	
	recordingKey := fmt.Sprintf(RecordingKeyPattern, recordingID)
	data, err := r.client.Get(ctx, recordingKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("recording session not found")
		}
		return nil, fmt.Errorf("failed to get recording session: %w", err)
	}
	
	var recordingData RecordingSessionData
	if err := json.Unmarshal([]byte(data), &recordingData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal recording session data: %w", err)
	}
	
	return &recordingData, nil
}

func (r *RedisSessionService) UpdateRecordingSession(recordingID string, recordingData *RecordingSessionData) error {
	ctx := context.Background()
	
	data, err := json.Marshal(recordingData)
	if err != nil {
		return fmt.Errorf("failed to marshal recording session data: %w", err)
	}
	
	recordingKey := fmt.Sprintf(RecordingKeyPattern, recordingID)
	ttl := r.client.TTL(ctx, recordingKey).Val()
	if ttl > 0 {
		return r.client.Set(ctx, recordingKey, data, ttl).Err()
	}
	
	return r.client.Set(ctx, recordingKey, data, 24*time.Hour).Err()
}

func (r *RedisSessionService) EndRecordingSession(recordingID string) error {
	recordingData, err := r.GetRecordingSession(recordingID)
	if err != nil {
		return err
	}
	
	now := time.Now()
	recordingData.EndedAt = &now
	recordingData.Duration = int64(now.Sub(recordingData.StartedAt).Seconds())
	recordingData.State = RecordingStateCompleted
	recordingData.IsActive = false
	
	return r.UpdateRecordingSession(recordingID, recordingData)
}

func (r *RedisSessionService) GetPodcastRecordings(podcastID string) ([]*RecordingSessionData, error) {
	ctx := context.Background()
	
	podcastRecordingsKey := fmt.Sprintf(PodcastRecordingsPattern, podcastID)
	recordingIDs, err := r.client.SMembers(ctx, podcastRecordingsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get podcast recordings: %w", err)
	}
	
	var recordings []*RecordingSessionData
	for _, recordingID := range recordingIDs {
		recording, err := r.GetRecordingSession(recordingID)
		if err != nil {
			log.Printf("Warning: failed to get recording %s: %v", recordingID, err)
			continue
		}
		recordings = append(recordings, recording)
	}
	
	return recordings, nil
}

func (r *RedisSessionService) AddParticipantToRecording(recordingID, userID string) error {
	recordingData, err := r.GetRecordingSession(recordingID)
	if err != nil {
		return err
	}
	
	for _, participant := range recordingData.Participants {
		if participant == userID {
			return nil
		}
	}
	
	recordingData.Participants = append(recordingData.Participants, userID)
	recordingData.ParticipantChunks[userID] = 0
	
	return r.UpdateRecordingSession(recordingID, recordingData)
}

func (r *RedisSessionService) UpdateParticipantChunkCount(recordingID, userID string, chunkCount int) error {
	recordingData, err := r.GetRecordingSession(recordingID)
	if err != nil {
		return err
	}
	
	recordingData.ParticipantChunks[userID] = chunkCount
	
	totalChunks := 0
	for _, count := range recordingData.ParticipantChunks {
		totalChunks += count
	}
	recordingData.TotalChunks = totalChunks
	
	return r.UpdateRecordingSession(recordingID, recordingData)
}

func (r *RedisSessionService) SetPodcastHost(podcastID, hostUserID string) error {
	ctx := context.Background()
	key := "podcast_host:" + podcastID
	return r.client.Set(ctx, key, hostUserID, 24*time.Hour).Err()
}

func (r *RedisSessionService) GetPodcastHost(podcastID string) (string, error) {
	ctx := context.Background()
	key := "podcast_host:" + podcastID
	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return result, nil
}



