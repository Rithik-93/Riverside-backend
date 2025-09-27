package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
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

	db := database.Connect()

	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	accessSecret := "secret"
	refreshSecret := "secret"

	tokenService := service.NewTokenService(
		accessSecret,
		refreshSecret,
	)
	oauthService := service.NewOAuthService(
		os.Getenv("GOOGLE_CLIENT_ID"),
		os.Getenv("GOOGLE_CLIENT_SECRET"),
		os.Getenv("GOOGLE_REDIRECT_URL"),
	)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	redisSessionService := infrastructure.NewRedisSessionService(redisClient)
	authService := domain.NewAuthService(userRepo, sessionRepo, tokenService, oauthService, redisSessionService)
	
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

	auth := router.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/oauth", authHandler.OAuthLogin)
		auth.GET("/validate", authHandler.ValidateToken)
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