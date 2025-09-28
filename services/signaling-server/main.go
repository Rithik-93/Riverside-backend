package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lakeside/services/signaling-server/config"
	"github.com/lakeside/services/signaling-server/handlers"
	"github.com/lakeside/services/signaling-server/internal"
)

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Hardcoded JWT secret for development
	jwtSecret := "secret"
	
	secretLength := 10
	if len(jwtSecret) < 10 {
		secretLength = len(jwtSecret)
	}
	log.Printf("Signaling server using JWT secret: %s...", jwtSecret[:secretLength])

	redisClient := config.NewRedisClient(redisAddr, os.Getenv("REDIS_PASSWORD"), 0)
	tokenService := internal.NewTokenService(jwtSecret)
	server := handlers.NewSignalingServer(redisClient, tokenService)

	router := gin.Default()

	corsConfig := cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}
	router.Use(cors.New(corsConfig))

	router.Use(func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "OK"})
	})

	router.GET("/ws", server.HandleWebSocket)

	router.GET("/podcasts/:podcastId", server.GetPodcastInfo)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Signaling server starting on port %s", port)
	log.Fatal(router.Run(":" + port))
}