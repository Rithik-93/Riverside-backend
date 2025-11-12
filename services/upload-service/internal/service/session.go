package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	eventspb "github.com/lakeside/backend/protos/gen/events"
	"github.com/lakeside/services/upload-service/monitoring"
	"github.com/lakeside/services/upload-service/pkg/types"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type SessionManager struct {
	sessions    map[string]*types.RecordingSession
	mutex       sync.RWMutex
	redisClient *redis.Client
	bucket      string
}

func NewSessionManager(redisClient *redis.Client, bucket string) *SessionManager {
	return &SessionManager{
		sessions:    make(map[string]*types.RecordingSession),
		redisClient: redisClient,
		bucket:      bucket,
	}
}

func (sm *SessionManager) TrackChunk(userID, podcastID, recordingID, s3Key, fileName, timestamp string, isFinal bool, chunkIndex int) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sessionKey := fmt.Sprintf("%s_%s", userID, podcastID)
	
	session, exists := sm.sessions[sessionKey]
	if !exists {
		session = &types.RecordingSession{
			UserID:      userID,
			PodcastID:   podcastID,
			RecordingID: recordingID,
			SessionID:   recordingID,
			Chunks:      make([]types.ChunkMetadata, 0),
			StartTime:   time.Now(),
			IsComplete:  false,
			State:       "started",
		}
		sm.sessions[sessionKey] = session
	}

	if session.State == "revoked" {
		log.Printf("Session revoked, rejecting chunk upload: User=%s, Podcast=%s", userID, podcastID)
		return
	}

	if session.State == "finalizing" && !isFinal {
		log.Printf("Session finalizing, rejecting non-final chunk: User=%s, Podcast=%s", userID, podcastID)
		return
	}

	actualChunkIndex := len(session.Chunks)
	if chunkIndex >= 0 {
		actualChunkIndex = chunkIndex
	}
	
	chunkMeta := types.ChunkMetadata{
		S3Key:       s3Key,
		RecordingID: recordingID,
		PodcastID:   podcastID,
		UserID:      userID,
		Timestamp:   timestamp,
		IsFinal:     isFinal,
		FileName:    fileName,
		ChunkIndex:  actualChunkIndex,
		UploadedAt:  time.Now(),
	}
	
	session.Chunks = append(session.Chunks, chunkMeta)
	session.LastChunkAt = time.Now()
	
	monitoring.Logger.LogChunkUploaded(userID, podcastID, s3Key, fileName, chunkMeta.FileSize, chunkMeta.ChunkIndex, isFinal)
	
	log.Printf("Recording chunk tracked: User=%s, Podcast=%s, S3Key=%s, IsFinal=%v, ChunkCount=%d, State=%s", 
		userID, podcastID, s3Key, isFinal, len(session.Chunks), session.State)

	if isFinal {
		session.State = "finalizing"
		session.IsComplete = true
		session.GracePeriodEnd = time.Now().Add(11 * time.Second)
		
		go sm.monitorGracePeriod(sessionKey, session)
		sm.notifyRecordingComplete(session)
		
		log.Printf("Recording session finalizing for user %s in podcast %s with %d chunks", 
			userID, podcastID, len(session.Chunks))
	}
}

func (sm *SessionManager) GetSession(userID, podcastID string) (*types.RecordingSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	sessionKey := fmt.Sprintf("%s_%s", userID, podcastID)
	session, exists := sm.sessions[sessionKey]
	return session, exists
}

func (sm *SessionManager) GetSessionByID(uploadID, userID string) (*types.RecordingSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	for _, s := range sm.sessions {
		if s.SessionID == uploadID && s.UserID == userID {
			return s, true
		}
	}
	return nil, false
}

func (sm *SessionManager) CreateSession(userID, podcastID, recordingID, uploadID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sessionKey := fmt.Sprintf("%s_%s", userID, podcastID)
	
	if _, exists := sm.sessions[sessionKey]; exists {
		return fmt.Errorf("upload session already exists for this podcast")
	}

	session := &types.RecordingSession{
		UserID:      userID,
		PodcastID:   podcastID,
		RecordingID: recordingID,
		SessionID:   uploadID,
		Chunks:      make([]types.ChunkMetadata, 0),
		StartTime:   time.Now(),
		IsComplete:  false,
	}
	sm.sessions[sessionKey] = session
	return nil
}

func (sm *SessionManager) RevokeSession(userID, podcastID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	sessionKey := fmt.Sprintf("%s_%s", userID, podcastID)
	session, exists := sm.sessions[sessionKey]
	if !exists {
		return fmt.Errorf("session not found")
	}
	
	if session.State != "completed" {
		session.State = "revoked"
		log.Printf("Session revoked: User=%s, Podcast=%s", userID, podcastID)
	}
	
	return nil
}

func (sm *SessionManager) monitorGracePeriod(sessionKey string, session *types.RecordingSession) {
	time.Sleep(11 * time.Second)
	
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	if existingSession, exists := sm.sessions[sessionKey]; exists && existingSession.State == "finalizing" {
		existingSession.State = "completed"
		log.Printf("Grace period ended, session completed: %s", sessionKey)
		
		go func() {
			time.Sleep(1 * time.Minute)
			sm.mutex.Lock()
			defer sm.mutex.Unlock()
			delete(sm.sessions, sessionKey)
			log.Printf("Session cleaned up: %s", sessionKey)
		}()
	}
}

func (sm *SessionManager) notifyRecordingComplete(session *types.RecordingSession) {
	if sm.redisClient == nil {
		log.Printf("Redis client not available, cannot notify recording completion")
		return
	}

	var podcastID, recordingID string
	if len(session.Chunks) > 0 {
		podcastID = session.Chunks[0].PodcastID
		recordingID = session.Chunks[0].RecordingID
	}

	chunkFolder := fmt.Sprintf("uploads/recordings/%s/%s/%s/chunks/", podcastID, recordingID, session.UserID)

	protoEvent := &eventspb.RedisEvent{
		EventType: "recording_complete",
		UserId:    session.UserID,
		ClientId:  "",
		PodcastId: session.PodcastID,
		Timestamp: time.Now().Unix(),
		Data: &eventspb.RedisEvent_RecordingComplete{
			RecordingComplete: &eventspb.RecordingCompleteData{
				SessionId:   session.SessionID,
				RecordingId: recordingID,
				PodcastId:   podcastID,
				TotalChunks: int32(len(session.Chunks)),
				S3Bucket:    sm.bucket,
				S3Region:    os.Getenv("AWS_REGION"),
				S3Endpoint:  os.Getenv("S3_ENDPOINT"),
				StartTime:   session.StartTime.Unix(),
				EndTime:     time.Now().Unix(),
				Duration:    time.Since(session.StartTime).Seconds(),
				OutputPath:  fmt.Sprintf("uploads/recordings/%s/%s/%s/final/final_recording_%s.webm", podcastID, recordingID, session.UserID, session.SessionID),
				ContentType: "video/webm",
				ChunkFolder: chunkFolder,
			},
		},
	}

	data, err := proto.Marshal(protoEvent)
	if err != nil {
		log.Printf("Failed to marshal protobuf event: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := sm.redisClient.LPush(ctx, "queue", data).Err(); err != nil {
		log.Printf("Failed to queue Redis event: %v", err)
	} else {
		log.Printf("✅ Recording completion notification sent to Redis (protobuf):")
		log.Printf("   - Session ID: %s", session.SessionID)
		log.Printf("   - User: %s, Podcast: %s", session.UserID, session.PodcastID)
		log.Printf("   - Chunk Folder: %s", chunkFolder)
		log.Printf("   - Duration: %.2f seconds", time.Since(session.StartTime).Seconds())
		log.Printf("   - Message size: %d bytes", len(data))
	}
}

// ValidateUserPermissions checks if user has permission for the podcast/recording
func ValidateUserPermissions(redisClient *redis.Client, userID, podcastID, recordingID string, isFinal bool) bool {
	log.Printf("🔍 DEBUG: validateUserPermissions called for user %s, podcast %s, recording %s", userID, podcastID, recordingID)
	
	if redisClient == nil {
		log.Printf("❌ SECURITY ERROR: Redis client not available - REJECTING upload for security")
		return false
	}

	if recordingID != "" {
		if !isRecordingValid(redisClient, recordingID, isFinal) {
			log.Printf("❌ Recording %s is not active or grace period expired", recordingID)
			return false
		}
		
		if validateRecordingParticipation(redisClient, userID, recordingID) {
			log.Printf("✅ User %s is participating in recording %s", userID, recordingID)
			return true
		}
		log.Printf("❌ User %s is NOT participating in recording %s", userID, recordingID)
	}
	
	if podcastID != "" {
		log.Printf("🔍 DEBUG: Checking podcast host for user %s in podcast %s", userID, podcastID)
		if validatePodcastHost(redisClient, userID, podcastID) {
			log.Printf("✅ User %s is host of podcast %s", userID, podcastID)
			return true
		}
		log.Printf("❌ User %s is NOT host of podcast %s", userID, podcastID)
	}
	
	if podcastID != "" && recordingID != "" {
		log.Printf("🔍 DEBUG: Checking recording in podcast for user %s, recording %s, podcast %s", userID, recordingID, podcastID)
		if validateRecordingInPodcast(redisClient, userID, podcastID, recordingID) {
			log.Printf("✅ User %s has permission for recording %s in podcast %s", userID, recordingID, podcastID)
			return true
		}
		log.Printf("❌ User %s does NOT have permission for recording %s in podcast %s", userID, recordingID, podcastID)
	}

	log.Printf("❌ No permission found for user %s (podcast: %s, recording: %s)", userID, podcastID, recordingID)
	return false
}

func isRecordingValid(redisClient *redis.Client, recordingID string, isFinal bool) bool {
	ctx := context.Background()
	
	recordingKey := fmt.Sprintf("recording:%s", recordingID)
	data, err := redisClient.Get(ctx, recordingKey).Result()
	if err != nil {
		if err == redis.Nil {
			log.Printf("Recording %s not found in Redis, rejecting upload", recordingID)
			return false
		}
		log.Printf("Failed to get recording %s: %v", recordingID, err)
		return false
	}
	
	var recordingData struct {
		IsActive bool       `json:"is_active"`
		EndedAt  *time.Time `json:"ended_at,omitempty"`
	}
	if err := json.Unmarshal([]byte(data), &recordingData); err != nil {
		log.Printf("Failed to deserialize recording data: %v", err)
		return false
	}
	
	if recordingData.IsActive {
		return true
	}
	
	if recordingData.EndedAt != nil && isFinal {
		gracePeriodEnd := recordingData.EndedAt.Add(11 * time.Second)
		if time.Now().Before(gracePeriodEnd) {
			log.Printf("Recording ended but within grace period, allowing final chunk")
			return true
		}
		log.Printf("Recording ended and grace period expired")
	}
	
	return false
}

func validateRecordingParticipation(redisClient *redis.Client, userID, recordingID string) bool {
	ctx := context.Background()
	
	recordingKey := fmt.Sprintf("recording:%s", recordingID)
	log.Printf("🔍 DEBUG: Looking for recording data with key: %s", recordingKey)
	data, err := redisClient.Get(ctx, recordingKey).Result()
	if err != nil {
		log.Printf("❌ Failed to get recording data for recording %s: %v", recordingID, err)
		return false
	}

	var recordingData struct {
		Participants []string `json:"participants"`
		PodcastID    string   `json:"podcast_id"`
		HostUserID   string   `json:"host_user_id"`
	}
	if err := json.Unmarshal([]byte(data), &recordingData); err != nil {
		log.Printf("Failed to parse recording data: %v", err)
		return false
	}

	for _, participant := range recordingData.Participants {
		if participant == userID {
			log.Printf("User %s in participants list", userID)
			return true
		}
	}
	
	log.Printf("🔍 DEBUG: Podcast ID from recording data: %s", recordingData.PodcastID)
	if recordingData.PodcastID != "" {
		if validateUserInPodcast(redisClient, userID, recordingData.PodcastID) {
			log.Printf("User %s is in podcast %s (fallback check)", userID, recordingData.PodcastID)
			return true
		}
	}
	
	log.Printf("❌ User %s is NOT participating in recording %s", userID, recordingID)
	return false
}

func validatePodcastHost(redisClient *redis.Client, userID, podcastID string) bool {
	ctx := context.Background()
	
	podcastSessionsPattern := "podcast_session:*"
	keys, err := redisClient.Keys(ctx, podcastSessionsPattern).Result()
	if err != nil {
		log.Printf("Failed to get podcast sessions: %v", err)
		return false
	}
	
	for _, key := range keys {
		sessionData, err := redisClient.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}
		
		if sessionData["podcast_id"] == podcastID && sessionData["user_id"] == userID {
			return true
		}
	}
	
	return false
}

func validateUserInPodcast(redisClient *redis.Client, userID, podcastID string) bool {
	ctx := context.Background()
	
	podcastSessionsPattern := "podcast_session:*"
	keys, err := redisClient.Keys(ctx, podcastSessionsPattern).Result()
	if err != nil {
		log.Printf("Failed to get podcast sessions: %v", err)
		return false
	}
	
	for _, key := range keys {
		sessionData, err := redisClient.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}
		
		if sessionData["podcast_id"] == podcastID && sessionData["user_id"] == userID {
			return true
		}
	}
	
	return false
}

func validateRecordingInPodcast(redisClient *redis.Client, userID, podcastID, recordingID string) bool {
	ctx := context.Background()
	
	recordingKey := fmt.Sprintf("recording:%s", recordingID)
	data, err := redisClient.Get(ctx, recordingKey).Result()
	if err != nil {
		log.Printf("Failed to get recording data for recording %s: %v", recordingID, err)
		return false
	}
	
	var recordingData struct {
		PodcastID string `json:"podcast_id"`
	}
	if err := json.Unmarshal([]byte(data), &recordingData); err != nil {
		log.Printf("Failed to parse recording data: %v", err)
		return false
	}
	
	if recordingData.PodcastID != podcastID {
		log.Printf("Recording %s does not belong to podcast %s", recordingID, podcastID)
		return false
	}
	
	return validateRecordingParticipation(redisClient, userID, recordingID)
}
