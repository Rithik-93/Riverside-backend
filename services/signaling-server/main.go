package main

import (
	"log"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lakeside/services/signaling-server/config"
	"github.com/lakeside/services/signaling-server/handlers"
	"github.com/lakeside/services/signaling-server/internal"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or could not be loaded: %v", err)
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	jwtSecret := os.Getenv("JWT_ACCESS_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_ACCESS_SECRET environment variable is required")
	}

	redisClient := config.NewRedisClient(redisAddr, os.Getenv("REDIS_PASSWORD"), 0)
	tokenService := internal.NewTokenService(jwtSecret)
	server := handlers.NewSignalingServer(redisClient, tokenService)

	router := gin.Default()

    allowedOriginsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
    if allowedOriginsEnv == "" {
        log.Fatal("CORS_ALLOWED_ORIGINS environment variable is required")
    }
    var origins []string
    for _, v := range strings.Split(allowedOriginsEnv, ",") {
        if o := strings.TrimSpace(v); o != "" {
            origins = append(origins, o)
        }
    }
    corsConfig := cors.Config{
        AllowOrigins:     origins,
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Content-Type", "Authorization", "Cookie"},
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