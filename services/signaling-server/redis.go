package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

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

func (r *RedisClient) QueueEvent(event *RedisEvent) {
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
	r.QueueEvent(&RedisEvent{
		EventType: EventClientConnected,
		UserID:    userID,
		ClientID:  clientID,
		Data: map[string]interface{}{
			"userAgent": userAgent,
			"ipAddress": ipAddress,
		},
	})
}

func (r *RedisClient) QueueClientDisconnected(clientID, userID string, duration int64) {
	r.QueueEvent(&RedisEvent{
		EventType: EventClientDisconnected,
		UserID:    userID,
		ClientID:  clientID,
		Data: map[string]interface{}{
			"duration": duration,
		},
	})
}

func (r *RedisClient) QueueRoomJoined(clientID, userID, roomID string) {
	r.QueueEvent(&RedisEvent{
		EventType: EventRoomJoined,
		UserID:    userID,
		ClientID:  clientID,
		RoomID:    roomID,
	})
}

func (r *RedisClient) QueueRoomLeft(clientID, userID, roomID string) {
	r.QueueEvent(&RedisEvent{
		EventType: EventRoomLeft,
		UserID:    userID,
		ClientID:  clientID,
		RoomID:    roomID,
	})
}

func (r *RedisClient) QueueCallStarted(clientID, userID, roomID, targetUserID string) {
	r.QueueEvent(&RedisEvent{
		EventType: EventCallStarted,
		UserID:    userID,
		ClientID:  clientID,
		RoomID:    roomID,
		Data: map[string]interface{}{
			"targetUserId": targetUserID,
		},
	})
}

func (r *RedisClient) QueueCallEnded(clientID, userID, roomID string, duration int64) {
	r.QueueEvent(&RedisEvent{
		EventType: EventCallEnded,
		UserID:    userID,
		ClientID:  clientID,
		RoomID:    roomID,
		Data: map[string]interface{}{
			"duration": duration,
		},
	})
}
