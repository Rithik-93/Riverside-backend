package infrastructure

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type RedisConsumer struct {
	client *redis.Client
	db     *gorm.DB
	logger *log.Logger
}

type SignalingEvent struct {
	EventType string                 `json:"eventType"`
	UserID    string                 `json:"userId,omitempty"`
	ClientID  string                 `json:"clientId"`
	RoomID    string                 `json:"roomId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

const (
	EventClientConnected    = "client_connected"
	EventClientDisconnected = "client_disconnected"
	EventRoomJoined         = "room_joined"
	EventRoomLeft           = "room_left"
	EventCallStarted        = "call_started"
	EventCallEnded          = "call_ended"
	EventRecordingStarted   = "recording_started"
	EventRecordingStopped   = "recording_stopped"
)

func NewRedisConsumer(db *gorm.DB) *RedisConsumer {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection failed: %v", err)
		return nil
	}

	log.Println("Redis consumer connected")
	return &RedisConsumer{
		client: rdb,
		db:     db,
		logger: log.New(os.Stdout, "[REDIS-CONSUMER] ", log.LstdFlags),
	}
}

func (r *RedisConsumer) StartConsuming() {
	if r == nil || r.client == nil {
		log.Println("Redis consumer not available, skipping event processing")
		return
	}

	log.Println("Starting Redis event consumer...")

	for {
		ctx := context.Background()

		result, err := r.client.BRPop(ctx, 0, "signaling_events_queue").Result()
		if err != nil {
			r.logger.Printf("Error consuming from Redis queue: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if len(result) < 2 {
			continue
		}

		var event SignalingEvent
		if err := json.Unmarshal([]byte(result[1]), &event); err != nil {
			r.logger.Printf("Failed to parse event: %v", err)
			continue
		}

		if err := r.processEvent(&event); err != nil {
			r.logger.Printf("Failed to process event: %v", err)
		}
	}
}

func (r *RedisConsumer) processEvent(event *SignalingEvent) error {
	r.logger.Printf("Processing event: %s for user: %s", event.EventType, event.UserID)

	switch event.EventType {
	case EventClientConnected:
		return r.handleClientConnected(event)
	case EventClientDisconnected:
		return r.handleClientDisconnected(event)
	case EventRoomJoined:
		return r.handleRoomJoined(event)
	case EventRoomLeft:
		return r.handleRoomLeft(event)
	case EventCallStarted:
		return r.handleCallStarted(event)
	case EventCallEnded:
		return r.handleCallEnded(event)
	case EventRecordingStarted:
		return r.handleRecordingStarted(event)
	case EventRecordingStopped:
		return r.handleRecordingStopped(event)
	default:
		r.logger.Printf("Unknown event type: %s", event.EventType)
		return nil
	}
}

func (r *RedisConsumer) handleClientConnected(event *SignalingEvent) error {
	r.logger.Printf("Client connected: %s for user: %s", event.ClientID, event.UserID)
	return nil
}

func (r *RedisConsumer) handleClientDisconnected(event *SignalingEvent) error {
	r.logger.Printf("Client disconnected: %s for user: %s", event.ClientID, event.UserID)
	return nil
}

func (r *RedisConsumer) handleRoomJoined(event *SignalingEvent) error {
	var pod Pod
	if err := r.db.Where("id = ?", event.RoomID).First(&pod).Error; err != nil {
		return err
	}

	participant := &PodParticipant{
		PodID:    pod.ID,
		UserID:   event.UserID,
		JoinedAt: time.Unix(event.Timestamp, 0),
	}

	return r.db.Where("pod_id = ? AND user_id = ?", pod.ID, event.UserID).
		Assign(participant).FirstOrCreate(participant).Error
}

func (r *RedisConsumer) handleRoomLeft(event *SignalingEvent) error {
	var pod Pod
	if err := r.db.Where("id = ?", event.RoomID).First(&pod).Error; err != nil {
		return err
	}

	return r.db.Where("pod_id = ? AND user_id = ?", pod.ID, event.UserID).
		Delete(&PodParticipant{}).Error
}

func (r *RedisConsumer) handleCallStarted(event *SignalingEvent) error {
	targetUserID := getStringFromData(event.Data, "targetUserId")
	
	pod := &Pod{
		HostUserID:       event.UserID,
		TargetUserID:     &targetUserID,
		ParticipantCount: 1,
		Status:           StatusOngoing,
		IsRecording:      false,
		StartedAt:        time.Unix(event.Timestamp, 0),
		IsActive:         true,
	}

	if err := r.db.Create(pod).Error; err != nil {
		return err
	}

	participant := &PodParticipant{
		PodID:    pod.ID,
		UserID:   event.UserID,
		JoinedAt: time.Unix(event.Timestamp, 0),
	}

	return r.db.Create(participant).Error
}

func (r *RedisConsumer) handleCallEnded(event *SignalingEvent) error {
	return r.db.Model(&Pod{}).
		Where("host_user_id = ? AND is_active = ?", event.UserID, true).
		Updates(map[string]interface{}{
			"status":     StatusCompleted,
			"ended_at":   time.Now(),
			"is_active":  false,
		}).Error
}

func getStringFromData(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt64FromData(data map[string]interface{}, key string) int64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case float64:
			return int64(v)
		case int:
			return int64(v)
		}
	}
	return 0
}

func (r *RedisConsumer) handleRecordingStarted(event *SignalingEvent) error {
	var pod Pod
	if err := r.db.Where("id = ?", event.RoomID).First(&pod).Error; err != nil {
		return err
	}

	return r.db.Model(&pod).Update("is_recording", true).Error
}

func (r *RedisConsumer) handleRecordingStopped(event *SignalingEvent) error {
	var pod Pod
	if err := r.db.Where("id = ?", event.RoomID).First(&pod).Error; err != nil {
		return err
	}

	return r.db.Model(&pod).Update("is_recording", false).Error
}
