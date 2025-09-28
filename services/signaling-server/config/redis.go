package config

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/lakeside/services/signaling-server/types"
	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(addr, password string, db int) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection failed: %v", err)
		return nil
	}

	log.Println("Connected to Redis")
	return &RedisClient{client: rdb}
}

func (r *RedisClient) QueueEvent(event *types.RedisEvent) {
	if r == nil || r.client == nil {
		return
	}

	event.Timestamp = time.Now().Unix()

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal Redis event: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := r.client.LPush(ctx, "signaling_events_queue", data).Err(); err != nil {
		log.Printf("Failed to queue Redis event: %v", err)
	}
}

func (r *RedisClient) QueueClientConnected(clientID, userID, userAgent, ipAddress string) {
	r.QueueEvent(&types.RedisEvent{
		EventType: types.EventClientConnected,
		UserID:    userID,
		ClientID:  clientID,
		Data: map[string]interface{}{
			"userAgent": userAgent,
			"ipAddress": ipAddress,
		},
	})
}

func (r *RedisClient) QueueClientDisconnected(clientID, userID string, duration int64) {
	r.QueueEvent(&types.RedisEvent{
		EventType: types.EventClientDisconnected,
		UserID:    userID,
		ClientID:  clientID,
		Data: map[string]interface{}{
			"duration": duration,
		},
	})
}

func (r *RedisClient) QueuePodcastJoined(clientID, userID, podcastID string) {
	r.QueueEvent(&types.RedisEvent{
		EventType: types.EventPodcastJoined,
		UserID:    userID,
		ClientID:  clientID,
		PodcastID: podcastID,
	})
}

func (r *RedisClient) QueuePodcastLeft(clientID, userID, podcastID string) {
	r.QueueEvent(&types.RedisEvent{
		EventType: types.EventPodcastLeft,
		UserID:    userID,
		ClientID:  clientID,
		PodcastID: podcastID,
	})
}

func (r *RedisClient) QueueRecordingStarted(clientID, userID, podcastID string) {
	r.QueueEvent(&types.RedisEvent{
		EventType: types.EventRecordingStarted,
		UserID:    userID,
		ClientID:  clientID,
		PodcastID: podcastID,
	})
}

func (r *RedisClient) QueueRecordingStopped(clientID, userID, podcastID string) {
	r.QueueEvent(&types.RedisEvent{
		EventType: types.EventRecordingStopped,
		UserID:    userID,
		ClientID:  clientID,
		PodcastID: podcastID,
	})
}

func (r *RedisClient) GetPodcastHost(podcastID string) (string, error) {
	if r == nil || r.client == nil {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := "podcast_host:" + podcastID
	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // Podcast doesn't exist
		}
		return "", err
	}

	return result, nil
}

func (r *RedisClient) SetPodcastHost(podcastID, userID string) error {
	if r == nil || r.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := "podcast_host:" + podcastID
	// Set with 24 hour expiration
	return r.client.Set(ctx, key, userID, 24*time.Hour).Err()
}

func (r *RedisClient) QueueCallStarted(clientID, userID, podcastID, targetUserID string) {
	r.QueueEvent(&types.RedisEvent{
		EventType: types.EventCallStarted,
		UserID:    userID,
		ClientID:  clientID,
		PodcastID: podcastID,
		Data: map[string]interface{}{
			"targetUserId": targetUserID,
		},
	})
}

func (r *RedisClient) QueueCallEnded(clientID, userID, podcastID string, duration int64) {
	r.QueueEvent(&types.RedisEvent{
		EventType: types.EventCallEnded,
		UserID:    userID,
		ClientID:  clientID,
		PodcastID: podcastID,
		Data: map[string]interface{}{
			"duration": duration,
		},
	})
}

func (r *RedisClient) CreatePodcastSession(sessionID, userID, podcastID string) error {
	if r == nil || r.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sessionData := map[string]interface{}{
		"session_id": sessionID,
		"user_id":    userID,
		"podcast_id": podcastID,
		"created_at": time.Now().Unix(),
		"type":       "podcast",
	}

	key := "podcast_session:" + sessionID
	return r.client.HSet(ctx, key, sessionData).Err()
}

func (r *RedisClient) CreateRecordingSession(sessionID, userID, podcastID, recordingID string) error {
	if r == nil || r.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sessionData := map[string]interface{}{
		"session_id":        sessionID,
		"user_id":           userID,
		"podcast_id":        podcastID,
		"recording_id":      recordingID,
		"created_at":        time.Now().Unix(),
		"type":              "recording",
	}

	key := "recording_session:" + sessionID
	return r.client.HSet(ctx, key, sessionData).Err()
}

func (r *RedisClient) CreateRecordingSessionWithParticipants(sessionID, userID, podcastID, recordingID string, participants []string) error {
	if r == nil || r.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	participantsJSON, err := json.Marshal(participants)
	if err != nil {
		log.Printf("Failed to marshal participants: %v", err)
		participantsJSON = []byte("[]")
	}

	sessionData := map[string]interface{}{
		"session_id":        sessionID,
		"host_user_id":      userID,
		"podcast_id":        podcastID,
		"recording_id":      recordingID,
		"participants":      string(participantsJSON),
		"created_at":        time.Now().Unix(),
		"type":              "recording",
	}

	key := "recording_session:" + sessionID
	return r.client.HSet(ctx, key, sessionData).Err()
}

func (r *RedisClient) DeleteSession(sessionID, sessionType string) error {
	if r == nil || r.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := sessionType + "_session:" + sessionID
	return r.client.Del(ctx, key).Err()
}

func (r *RedisClient) DeleteUserSession(sessionID, userID string) error {
	if r == nil || r.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sessionKey := "session:" + sessionID
	if err := r.client.Del(ctx, sessionKey).Err(); err != nil {
		log.Printf("Failed to delete session: %v", err)
		return err
	}

	userSessionsKey := "user_sessions:" + userID
	if err := r.client.SRem(ctx, userSessionsKey, sessionID).Err(); err != nil {
		log.Printf("Failed to remove session from user sessions set: %v", err)
		return err
	}

	log.Printf("User session deleted: %s for user: %s", sessionID, userID)
	return nil
}