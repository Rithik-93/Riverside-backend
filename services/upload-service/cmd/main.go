package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lakeside/services/upload-service/internal/handlers"
	"github.com/lakeside/services/upload-service/internal/middleware"
	"github.com/lakeside/services/upload-service/internal/service"
	"github.com/lakeside/services/upload-service/monitoring"
	"github.com/redis/go-redis/v9"
)

func initS3() (*s3.Client, string) {
	// Check for required AWS credentials
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if len(accessKey) == 0 || len(secretKey) == 0 {
		log.Fatal("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables are required")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" && region == "blr1" {
		endpoint = "https://blr1.digitaloceanspaces.com"
	}

	var cfg aws.Config
	var err error

	if endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           endpoint,
				SigningRegion: region,
			}, nil
		})

		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
			config.WithEndpointResolverWithOptions(customResolver),
		)
	} else {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(region),
		)
	}

	if err != nil {
		log.Fatal("Failed to load AWS config:", err)
	}

	s3Svc := s3.NewFromConfig(cfg)
	bucket := os.Getenv("S3_BUCKET_NAME")
	if bucket == "" {
		log.Fatal("S3_BUCKET_NAME environment variable is required")
	}

	log.Printf("S3 initialized - Bucket: %s, Region: %s, Endpoint: %s", bucket, region, endpoint)
	return s3Svc, bucket
}

func initRedis() *redis.Client {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection failed: %v", err)
		return nil
	}
	
	log.Println("Connected to Redis")
	return redisClient
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	jwtSecret := os.Getenv("JWT_ACCESS_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_ACCESS_SECRET environment variable is required")
	}
	middleware.SetJWTSecret(jwtSecret)

	s3Client, bucket := initS3()
	redisClient := initRedis()
	monitoring.InitializeAuditLogger()
	
	sessionManager := service.NewSessionManager(redisClient, bucket)
	uploadHandler := handlers.NewUploadHandler(s3Client, bucket, redisClient, sessionManager)

    r := gin.Default()

    allowedOriginsEnv := os.Getenv("CORS_ALLOWED_ORIGINS")
    if allowedOriginsEnv == "" {
        log.Fatal("CORS_ALLOWED_ORIGINS environment variable is required")
    }
    allowedOrigins := map[string]struct{}{}
    for _, v := range strings.Split(allowedOriginsEnv, ",") {
        origin := strings.TrimSpace(v)
        if origin != "" {
            allowedOrigins[origin] = struct{}{}
        }
    }

    r.Use(func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")
        if _, ok := allowedOrigins[origin]; ok {
            c.Header("Access-Control-Allow-Origin", origin)
        }
        c.Header("Access-Control-Allow-Credentials", "true")
        c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Cookie")
        c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    })

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	protected := r.Group("/api/v1")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.POST("/upload/presigned-url", uploadHandler.GetPreSignedURL)
		protected.POST("/upload/start", uploadHandler.StartUploadSession)
		protected.POST("/upload/finalize", uploadHandler.FinalizeUploadSession)
		protected.POST("/upload/revoke", uploadHandler.RevokeUploadSession)
		protected.GET("/upload/status/:uploadId", uploadHandler.GetUploadStatus)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(r.Run(":" + port))
}
