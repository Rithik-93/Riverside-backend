package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/lakeside/services/session-service/internal/auth"
	"github.com/lakeside/services/session-service/internal/domain"
	"github.com/lakeside/services/session-service/internal/handlers"
	"github.com/lakeside/services/session-service/internal/infrastructure"
	"github.com/lakeside/services/session-service/internal/infrastructure/database"
	"github.com/lakeside/services/session-service/internal/infrastructure/repository"
	"github.com/lakeside/services/session-service/internal/service"
	"github.com/lakeside/services/session-service/pkg"
	"github.com/redis/go-redis/v9"
)

func authMiddleware(tokenService *service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			c.JSON(401, gin.H{"error": "Access token required"})
			c.Abort()
			return
		}

		claims, err := tokenService.ValidateAccessToken(token)
		if err != nil {
			c.JSON(401, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_username", claims.Username)
		c.Next()
	}
}

func main() {
	pkg.LoadEnv()

	auth.NewAuth()

	db := database.Connect()

	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	accessSecret := "secret"
	refreshSecret := "secret"

	tokenService := service.NewTokenService(
		accessSecret,
		refreshSecret,
	)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	redisSessionService := infrastructure.NewRedisSessionService(redisClient)
	authService := domain.NewAuthService(userRepo, sessionRepo, tokenService, nil, redisSessionService)
	
	authHandler := handlers.NewAuthHandler(authService)

	redisConsumer := infrastructure.NewRedisConsumer(db)
	if redisConsumer != nil {
		go redisConsumer.StartConsuming()
		log.Println("Redis consumer started")
	}

	router := gin.Default()

	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "OK"})
	})

	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/logout", authHandler.Logout)
		authGroup.GET("/:provider", auth.BeginAuth)
		authGroup.GET("/:provider/callback", auth.AuthController)
		authGroup.GET("/validate", authHandler.ValidateToken)
	}

	sessions := router.Group("/sessions")
	sessions.Use(authMiddleware(tokenService))
	{
		sessions.GET("/:sessionId", authHandler.ValidateSession)
		sessions.DELETE("/:sessionId", authHandler.DeleteSession)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(router.Run(":" + port))
}