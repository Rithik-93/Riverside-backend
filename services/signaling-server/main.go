package main

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type SignalingServer struct {
	rooms       map[string]*Room
	clients     map[string]*Client
	redisClient *RedisClient
	mutex       sync.RWMutex
}

func NewSignalingServer() *SignalingServer {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient := NewRedisClient(redisAddr, os.Getenv("REDIS_PASSWORD"), 0)

	return &SignalingServer{
		rooms:       make(map[string]*Room),
		clients:     make(map[string]*Client),
		redisClient: redisClient,
	}
}

func main() {
	server := NewSignalingServer()

	router := gin.Default()

	router.Use(func(c *gin.Context) {

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "OK"})
	})

	router.GET("/ws", server.handleWebSocket)

	router.GET("/rooms/:roomId", func(c *gin.Context) {
		roomID := c.Param("roomId")

		server.mutex.RLock()
		room, exists := server.rooms[roomID]
		server.mutex.RUnlock()

		if !exists {
			c.JSON(404, gin.H{"error": "Room not found"})
			return
		}

		var clients []string
		for clientID := range room.Clients {
			clients = append(clients, clientID)
		}

		c.JSON(200, gin.H{
			"roomId":  roomID,
			"clients": clients,
			"count":   len(clients),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Signaling server starting on port %s", port)
	log.Fatal(router.Run(":" + port))
}
