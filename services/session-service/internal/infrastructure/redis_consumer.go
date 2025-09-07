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
	default:
		r.logger.Printf("Unknown event type: %s", event.EventType)
		return nil
	}
}

func (r *RedisConsumer) handleClientConnected(event *SignalingEvent) error {
	connection := &ClientConnection{
		ClientID:    event.ClientID,
		UserID:      event.UserID,
		ConnectedAt: time.Unix(event.Timestamp, 0),
		UserAgent:   getStringFromData(event.Data, "userAgent"),
		IPAddress:   getStringFromData(event.Data, "ipAddress"),
		IsActive:    true,
	}

	return r.db.Where("client_id = ?", event.ClientID).Assign(connection).FirstOrCreate(connection).Error
}

func (r *RedisConsumer) handleClientDisconnected(event *SignalingEvent) error {
	duration := getInt64FromData(event.Data, "duration")

	return r.db.Model(&ClientConnection{}).
		Where("client_id = ?", event.ClientID).
		Updates(map[string]interface{}{
			"is_active":       false,
			"disconnected_at": time.Now(),
			"duration_ms":     duration,
		}).Error
}

func (r *RedisConsumer) handleRoomJoined(event *SignalingEvent) error {
	participation := &RoomParticipation{
		UserID:   event.UserID,
		ClientID: event.ClientID,
		RoomID:   event.RoomID,
		JoinedAt: time.Unix(event.Timestamp, 0),
		IsActive: true,
	}

	return r.db.Where("user_id = ? AND room_id = ? AND client_id = ?",
		event.UserID, event.RoomID, event.ClientID).
		Assign(participation).FirstOrCreate(participation).Error
}

func (r *RedisConsumer) handleRoomLeft(event *SignalingEvent) error {
	return r.db.Model(&RoomParticipation{}).
		Where("user_id = ? AND room_id = ? AND client_id = ?",
			event.UserID, event.RoomID, event.ClientID).
		Updates(map[string]interface{}{
			"is_active": false,
			"left_at":   time.Now(),
		}).Error
}

func (r *RedisConsumer) handleCallStarted(event *SignalingEvent) error {
	targetUserID := getStringFromData(event.Data, "targetUserId")

	callSession := &CallSession{
		UserID:       event.UserID,
		ClientID:     event.ClientID,
		RoomID:       event.RoomID,
		TargetUserID: targetUserID,
		StartedAt:    time.Unix(event.Timestamp, 0),
		IsActive:     true,
	}

	return r.db.Create(callSession).Error
}

func (r *RedisConsumer) handleCallEnded(event *SignalingEvent) error {
	duration := getInt64FromData(event.Data, "duration")

	return r.db.Model(&CallSession{}).
		Where("user_id = ? AND room_id = ? AND client_id = ? AND is_active = ?",
			event.UserID, event.RoomID, event.ClientID, true).
		Updates(map[string]interface{}{
			"is_active":   false,
			"ended_at":    time.Now(),
			"duration_ms": duration,
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
