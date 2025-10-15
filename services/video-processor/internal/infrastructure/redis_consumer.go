package infrastructure

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConsumer struct {
	client *redis.Client
}

func NewRedisConsumer() *RedisConsumer {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("✅ Connected to Redis")

	return &RedisConsumer{client: client}
}

func (r *RedisConsumer) ConsumeQueue(queueName string) ([]byte, error) {
	ctx := context.Background()
	result, err := r.client.BRPop(ctx, 0, queueName).Result()
	if err != nil {
		return nil, err
	}

	if len(result) < 2 {
		return nil, nil
	}

	return []byte(result[1]), nil
}

