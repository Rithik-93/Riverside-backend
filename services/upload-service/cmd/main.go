package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	eventspb "github.com/lakeside/backend/protos/gen/events"
	"github.com/lakeside/services/upload-service/monitoring"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type TokenClaims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type PreSignedURLRequest struct {
	FileName     string `json:"file_name" binding:"required"`
	ContentType  string `json:"content_type" binding:"required"`
	UserID       string `json:"user_id,omitempty"`
	PodcastID    string `json:"podcast_id,omitempty"`
	RecordingID  string `json:"recording_id,omitempty"`
	IsFinal      bool   `json:"is_final,omitempty"`
	IsChunk      bool   `json:"is_chunk,omitempty"`
	Timestamp    string `json:"timestamp,omitempty"`
	ChunkIndex   int    `json:"chunk_index,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type PreSignedURLResponse struct {
	PreSignedURL string `json:"pre_signed_url"`
	S3Key        string `json:"s3_key"`
	ExpiresIn    int    `json:"expires_in"`
	UploadID     string `json:"upload_id,omitempty"`
	ChunkIndex   int    `json:"chunk_index,omitempty"`
}

type ChunkMetadata struct {
	S3Key        string    `json:"s3_key"`
	RecordingID  string    `json:"recording_id"`
	PodcastID    string    `json:"podcast_id"`
	UserID       string    `json:"user_id"`
	Timestamp    string    `json:"timestamp"`
	IsFinal      bool      `json:"is_final"`
	FileName     string    `json:"file_name"`
	ChunkIndex   int       `json:"chunk_index"`
	FileSize     int64     `json:"file_size"`
	UploadedAt   time.Time `json:"uploaded_at"`
	Checksum     string    `json:"checksum,omitempty"`
}

type RecordingSession struct {
	UserID         string          `json:"user_id"`
	PodcastID      string          `json:"podcast_id"`
	RecordingID    string          `json:"recording_id"`
	SessionID      string          `json:"session_id"`
	Chunks         []ChunkMetadata `json:"chunks"`
	StartTime      time.Time       `json:"start_time"`
	IsComplete     bool            `json:"is_complete"`
	State          string          `json:"state"` // started, finalizing, completed, revoked
	LastChunkAt    time.Time       `json:"last_chunk_at"`
	GracePeriodEnd time.Time       `json:"grace_period_end,omitempty"`
}

type RedisEvent struct {
	EventType string                 `json:"eventType"`
	UserID    string                 `json:"userId,omitempty"`
	ClientID  string                 `json:"clientId"`
	PodcastID string                 `json:"podcastId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

var (
	s3Svc            *s3.Client
	bucket           string
	redisClient      *redis.Client
	recordingSessions map[string]*RecordingSession
	sessionMutex     sync.RWMutex
	jwtSecret        string
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		var tokenString string
		
		if authHeader != "" {
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) == 2 && tokenParts[0] == "Bearer" {
				tokenString = tokenParts[1]
			}
		} else {
			cookie, err := c.Cookie("access_token")
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Access token required"})
				c.Abort()
				return
			}
			tokenString = cookie
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Access token required"})
			c.Abort()
			return
		}

		claims, err := validateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_username", claims.Username)
		c.Next()
	}
}

func validateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
			return nil, fmt.Errorf("token expired")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func initS3() {
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

	s3Svc = s3.NewFromConfig(cfg)
	bucket = os.Getenv("S3_BUCKET_NAME")
	if bucket == "" {
		log.Fatal("S3_BUCKET_NAME environment variable is required")
	}

	log.Printf("S3 initialized - Bucket: %s, Region: %s, Endpoint: %s", bucket, region, endpoint)
}

func initRedis() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection failed: %v", err)
		redisClient = nil
	} else {
		log.Println("Connected to Redis")
	}
}

func generateS3Key(fileName, userID, podcastID, recordingID string, isChunk bool) string {
	if userID != "" && podcastID != "" && recordingID != "" {
		// For recording chunks: uploads/recordings/{podcastID}/{recordingID}/{userID}/chunks/
		// All chunks go to chunks directory, final videos go to final directory (handled by video processor)
		return fmt.Sprintf("uploads/recordings/%s/%s/%s/chunks/%s", 
			podcastID, recordingID, userID, fileName)
	} else {
		// For regular uploads: use UUID for unique folder
		uuid := uuid.New().String()
		return fmt.Sprintf("uploads/files/%s/%s", uuid, fileName)
	}
}

func getPreSignedURL(c *gin.Context) {
	var req PreSignedURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authenticatedUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userID := authenticatedUserID.(string)
	req.UserID = userID

	if err := validatePresignedURLRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// SECURITY FIX: Require podcast_id or recording_id
	if req.PodcastID == "" && req.RecordingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "podcast_id or recording_id is required"})
		return
	}

	if !validateUserPermissions(userID, req.PodcastID, req.RecordingID) {
		ipAddress, userAgent := getClientInfo(c)
		monitoring.Logger.LogSessionValidationFailed(userID, req.PodcastID, "No permission for this podcast/recording", ipAddress, userAgent)
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission for this podcast/recording"})
		return
	}

	// TEMPORARY FIX: If chunk index is 0, use session chunk count instead
	actualChunkIndex := req.ChunkIndex
	if req.ChunkIndex == 0 && req.UserID != "" && req.PodcastID != "" {
		sessionKey := fmt.Sprintf("%s_%s", req.UserID, req.PodcastID)
		sessionMutex.RLock()
		if session, exists := recordingSessions[sessionKey]; exists {
			actualChunkIndex = len(session.Chunks)
			log.Printf("🔧 TEMP FIX: Using session chunk count %d instead of 0", actualChunkIndex)
		}
		sessionMutex.RUnlock()
	}
	
	s3Key := generateS3Key(req.FileName, userID, req.PodcastID, req.RecordingID, req.IsChunk)
	putObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(s3Key),
		ContentType: aws.String(req.ContentType),
		ChecksumAlgorithm: "SHA256", // Enable checksum validation
	}

	if req.FileSize > 0 {
		putObjectInput.ContentLength = aws.Int64(req.FileSize)
	}

	expirationTime := 3 * time.Minute
	presignClient := s3.NewPresignClient(s3Svc)
	preSignedURL, err := presignClient.PresignPutObject(context.TODO(), putObjectInput, func(opts *s3.PresignOptions) {
		opts.Expires = expirationTime
	})
	if err != nil {
		log.Printf("Failed to generate pre-signed URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate pre-signed URL"})
		return
	}

	if req.UserID != "" && req.PodcastID != "" && req.RecordingID != "" {
		trackRecordingChunk(req.UserID, req.PodcastID, req.RecordingID, s3Key, req.FileName, req.Timestamp, req.IsFinal, actualChunkIndex)
	}

	response := PreSignedURLResponse{
		PreSignedURL: preSignedURL.URL,
		S3Key:        s3Key,
		ExpiresIn:    int(expirationTime.Seconds()),
		ChunkIndex:   actualChunkIndex,
	}

	ipAddress, userAgent := getClientInfo(c)
	monitoring.Logger.LogPresignedURLIssued(userID, req.PodcastID, s3Key, req.FileName, req.ContentType, req.FileSize, actualChunkIndex, ipAddress, userAgent)

	log.Printf("Generated secure presigned URL for user %s, podcast %s, chunk %d: %s", 
		userID, req.PodcastID, actualChunkIndex, s3Key)
	c.JSON(http.StatusOK, response)
}

func validatePresignedURLRequest(req *PreSignedURLRequest) error {
	if req.FileName == "" {
		return fmt.Errorf("file name is required")
	}
	
	allowedContentTypes := map[string]bool{
		"video/webm": true,
		"video/mp4":  true,
		"audio/webm": true,
		"audio/mp4":  true,
	}
	
	if !allowedContentTypes[req.ContentType] {
		return fmt.Errorf("unsupported content type: %s", req.ContentType)
	}
	
	if req.FileSize > 0 && req.FileSize > 100*1024*1024 { // 100MB limit
		return fmt.Errorf("file size too large: %d bytes", req.FileSize)
	}
	
	if req.ChunkIndex < 0 {
		return fmt.Errorf("chunk index must be non-negative")
	}
	
	return nil
}

func validateUserPermissions(userID, podcastID, recordingID string) bool {
	log.Printf("🔍 DEBUG: validateUserPermissions called for user %s, podcast %s, recording %s", userID, podcastID, recordingID)
	
	// SECURITY FIX: Never skip validation - Redis must be available
	if redisClient == nil {
		log.Printf("❌ SECURITY ERROR: Redis client not available - REJECTING upload for security")
		return false // REJECT if Redis is not available
	}

	if recordingID != "" {
		log.Printf("🔍 DEBUG: Checking recording participation for user %s in recording %s", userID, recordingID)
		if validateRecordingParticipation(userID, recordingID) {
			log.Printf("✅ User %s is participating in recording %s", userID, recordingID)
			return true
		}
		log.Printf("❌ User %s is NOT participating in recording %s", userID, recordingID)
	}
	
	// If we have a podcastID, check if user is the host of that podcast
	if podcastID != "" {
		log.Printf("🔍 DEBUG: Checking podcast host for user %s in podcast %s", userID, podcastID)
		if validatePodcastHost(userID, podcastID) {
			log.Printf("✅ User %s is host of podcast %s", userID, podcastID)
			return true
		}
		log.Printf("❌ User %s is NOT host of podcast %s", userID, podcastID)
	}
	
	if podcastID != "" && recordingID != "" {
		log.Printf("🔍 DEBUG: Checking recording in podcast for user %s, recording %s, podcast %s", userID, recordingID, podcastID)
		if validateRecordingInPodcast(userID, podcastID, recordingID) {
			log.Printf("✅ User %s has permission for recording %s in podcast %s", userID, recordingID, podcastID)
			return true
		}
		log.Printf("❌ User %s does NOT have permission for recording %s in podcast %s", userID, recordingID, podcastID)
	}

	log.Printf("❌ No permission found for user %s (podcast: %s, recording: %s)", userID, podcastID, recordingID)
	return false
}

func validateRecordingParticipation(userID, recordingID string) bool {
	ctx := context.Background()
	
	recordingKey := fmt.Sprintf("recording_session:%s", recordingID)
	log.Printf("🔍 DEBUG: Looking for recording data with key: %s", recordingKey)
	recordingData, err := redisClient.HGetAll(ctx, recordingKey).Result()
	if err != nil {
		log.Printf("❌ Failed to get recording data for recording %s: %v", recordingID, err)
		return false
	}

	participants := recordingData["participants"]
	log.Printf("🔍 DEBUG: Participants data: %s", participants)
	if participants != "" {
		var participantList []string
		if err := json.Unmarshal([]byte(participants), &participantList); err == nil {
			log.Printf("🔍 DEBUG: Parsed participants list: %+v", participantList)
			for _, participant := range participantList {
				if participant == userID {
					log.Printf("✅ User %s found in participants list", userID)
					return true
				}
			}
		} else {
			log.Printf("❌ Failed to parse participants JSON: %v", err)
		}
	} else {
		log.Printf("❌ No participants data found in recording session")
	}
	
	// Fallback: Check if user is currently in a podcast session that matches this recording
	// This handles cases where participants might not be stored in the recording session
	podcastID := recordingData["podcast_id"]
	log.Printf("🔍 DEBUG: Podcast ID from recording data: %s", podcastID)
	if podcastID != "" {
		if validateUserInPodcast(userID, podcastID) {
			log.Printf("✅ User %s is in podcast %s (fallback check)", userID, podcastID)
			return true
		}
	}
	
	log.Printf("❌ User %s is NOT participating in recording %s", userID, recordingID)
	return false
}

func validatePodcastHost(userID, podcastID string) bool {
	ctx := context.Background()
	
	podcastSessionsPattern := "podcast_session:*"
	keys, err := redisClient.Keys(ctx, podcastSessionsPattern).Result()
	if err != nil {
		log.Printf("Failed to get podcast sessions: %v", err)
		return false
	}
	
	for _, key := range keys {
		sessionData, err := redisClient.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}
		
		if sessionData["podcast_id"] == podcastID && sessionData["user_id"] == userID {
			return true
		}
	}
	
	return false
}

func validateUserInPodcast(userID, podcastID string) bool {
	ctx := context.Background()
	
	podcastSessionsPattern := "podcast_session:*"
	keys, err := redisClient.Keys(ctx, podcastSessionsPattern).Result()
	if err != nil {
		log.Printf("Failed to get podcast sessions: %v", err)
		return false
	}
	
	for _, key := range keys {
		sessionData, err := redisClient.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}
		
		if sessionData["podcast_id"] == podcastID && sessionData["user_id"] == userID {
			return true
		}
	}
	
	return false
}

func validateRecordingInPodcast(userID, podcastID, recordingID string) bool {
	ctx := context.Background()
	
	recordingKey := fmt.Sprintf("recording_session:%s", recordingID)
	recordingData, err := redisClient.HGetAll(ctx, recordingKey).Result()
	if err != nil {
		log.Printf("Failed to get recording data for recording %s: %v", recordingID, err)
		return false
	}
	
	if recordingData["podcast_id"] != podcastID {
		log.Printf("Recording %s does not belong to podcast %s", recordingID, podcastID)
		return false
	}
	
	return validateRecordingParticipation(userID, recordingID)
}

func trackRecordingChunk(userID, podcastID, recordingID, s3Key, fileName, timestamp string, isFinal bool, chunkIndex int) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sessionKey := fmt.Sprintf("%s_%s", userID, podcastID)
	
	session, exists := recordingSessions[sessionKey]
	if !exists {
		session = &RecordingSession{
			UserID:      userID,
			PodcastID:   podcastID,
			RecordingID: recordingID,
			SessionID:   recordingID,
			Chunks:      make([]ChunkMetadata, 0),
			StartTime:   time.Now(),
			IsComplete:  false,
			State:       "started",
		}
		recordingSessions[sessionKey] = session
	}

	if session.State == "revoked" {
		log.Printf("Session revoked, rejecting chunk upload: User=%s, Podcast=%s", userID, podcastID)
		return
	}

	if session.State == "finalizing" && !isFinal {
		log.Printf("Session finalizing, rejecting non-final chunk: User=%s, Podcast=%s", userID, podcastID)
		return
	}

	actualChunkIndex := len(session.Chunks)
	if chunkIndex >= 0 {
		actualChunkIndex = chunkIndex
	}
	
	chunkMeta := ChunkMetadata{
		S3Key:       s3Key,
		RecordingID: recordingID,
		PodcastID:   podcastID,
		UserID:      userID,
		Timestamp:   timestamp,
		IsFinal:     isFinal,
		FileName:    fileName,
		ChunkIndex:  actualChunkIndex,
		UploadedAt:  time.Now(),
	}
	
	session.Chunks = append(session.Chunks, chunkMeta)
	session.LastChunkAt = time.Now()
	
	monitoring.Logger.LogChunkUploaded(userID, podcastID, s3Key, fileName, chunkMeta.FileSize, chunkMeta.ChunkIndex, isFinal)
	
	log.Printf("Recording chunk tracked: User=%s, Podcast=%s, S3Key=%s, IsFinal=%v, ChunkCount=%d, State=%s", 
		userID, podcastID, s3Key, isFinal, len(session.Chunks), session.State)

	if isFinal {
		session.State = "finalizing"
		session.IsComplete = true
		
		session.GracePeriodEnd = time.Now().Add(5 * time.Minute)
		
		go monitorGracePeriod(sessionKey, session)
		
		notifyRecordingComplete(session)
		log.Printf("Recording session finalizing for user %s in podcast %s with %d chunks", 
			userID, podcastID, len(session.Chunks))
	}
}

func monitorGracePeriod(sessionKey string, session *RecordingSession) {
	time.Sleep(5 * time.Minute)
	
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	if existingSession, exists := recordingSessions[sessionKey]; exists && existingSession.State == "finalizing" {
		existingSession.State = "completed"
		log.Printf("Grace period ended, session completed: %s", sessionKey)
		
		go func() {
			time.Sleep(1 * time.Minute)
			sessionMutex.Lock()
			defer sessionMutex.Unlock()
			delete(recordingSessions, sessionKey)
			log.Printf("Session cleaned up: %s", sessionKey)
		}()
	}
}

func revokeSession(userID, podcastID string) error {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	sessionKey := fmt.Sprintf("%s_%s", userID, podcastID)
	session, exists := recordingSessions[sessionKey]
	if !exists {
		return fmt.Errorf("session not found")
	}
	
	if session.State != "completed" {
		session.State = "revoked"
		log.Printf("Session revoked: User=%s, Podcast=%s", userID, podcastID)
	}
	
	return nil
}

func notifyRecordingComplete(session *RecordingSession) {
	if redisClient == nil {
		log.Printf("Redis client not available, cannot notify recording completion")
		return
	}

	var podcastID, recordingID string
	if len(session.Chunks) > 0 {
		podcastID = session.Chunks[0].PodcastID
		recordingID = session.Chunks[0].RecordingID
	}

	chunkFolder := fmt.Sprintf("uploads/recordings/%s/%s/%s/chunks/", podcastID, recordingID, session.UserID)

	protoEvent := &eventspb.RedisEvent{
		EventType: "recording_complete",
		UserId:    session.UserID,
		ClientId:  "", // No specific client for recording completion
		PodcastId: session.PodcastID,
		Timestamp: time.Now().Unix(),
		Data: &eventspb.RedisEvent_RecordingComplete{
			RecordingComplete: &eventspb.RecordingCompleteData{
				SessionId:   session.SessionID,
				RecordingId: recordingID,
				PodcastId:   podcastID,
				TotalChunks: int32(len(session.Chunks)),
				S3Bucket:    bucket,
				S3Region:    os.Getenv("AWS_REGION"),
				S3Endpoint:  os.Getenv("S3_ENDPOINT"),
				StartTime:   session.StartTime.Unix(),
				EndTime:     time.Now().Unix(),
				Duration:    time.Since(session.StartTime).Seconds(),
				OutputPath:  fmt.Sprintf("uploads/recordings/%s/%s/%s/final/final_recording_%s.webm", podcastID, recordingID, session.UserID, session.SessionID),
				ContentType: "video/webm",
				ChunkFolder: chunkFolder,
			},
		},
	}

	data, err := proto.Marshal(protoEvent)
	if err != nil {
		log.Printf("Failed to marshal protobuf event: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := redisClient.LPush(ctx, "queue", data).Err(); err != nil {
		log.Printf("Failed to queue Redis event: %v", err)
	} else {
		log.Printf("✅ Recording completion notification sent to Redis (protobuf):")
		log.Printf("   - Session ID: %s", session.SessionID)
		log.Printf("   - User: %s, Podcast: %s", session.UserID, session.PodcastID)
		log.Printf("   - Chunk Folder: %s", chunkFolder)
		log.Printf("   - Duration: %.2f seconds", time.Since(session.StartTime).Seconds())
		log.Printf("   - Message size: %d bytes", len(data))
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	jwtSecret = "secret"

	initS3()
	initRedis()
	monitoring.InitializeAuditLogger()
	
	recordingSessions = make(map[string]*RecordingSession)

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173") // Frontend URL
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Cookie")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
		}
		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	protected := r.Group("/api/v1")
	protected.Use(AuthMiddleware())
	{
		protected.POST("/upload/presigned-url", getPreSignedURL)
		protected.POST("/upload/start", startUploadSession)
		protected.POST("/upload/finalize", finalizeUploadSession)
		protected.POST("/upload/revoke", revokeUploadSession)
		protected.GET("/upload/status/:uploadId", getUploadStatus)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(r.Run(":" + port))
}

func startUploadSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := userID.(string)

	var req struct {
		PodcastID   string `json:"podcast_id" binding:"required"`
		RecordingID string `json:"recording_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "podcast_id is required"})
		return
	}

	if req.PodcastID == "" && req.RecordingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "podcast_id or recording_id is required"})
		return
	}

	if !validateUserPermissions(userIDStr, req.PodcastID, req.RecordingID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission for this podcast/recording"})
		return
	}

	// Create upload session
	uploadID := uuid.New().String()
	sessionKey := fmt.Sprintf("%s_%s", userIDStr, req.PodcastID)
	
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	if _, exists := recordingSessions[sessionKey]; exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Upload session already exists for this podcast"})
		return
	}

	session := &RecordingSession{
		UserID:      userIDStr,
		PodcastID:   req.PodcastID,
		RecordingID: req.RecordingID,
		SessionID:   uploadID,
		Chunks:      make([]ChunkMetadata, 0),
		StartTime:   time.Now(),
		IsComplete:  false,
	}
	recordingSessions[sessionKey] = session

	ipAddress, userAgent := getClientInfo(c)
	monitoring.Logger.LogUploadSessionStarted(userIDStr, req.PodcastID, uploadID, ipAddress, userAgent)

	log.Printf("Upload session started: %s for user %s, podcast %s", uploadID, userIDStr, req.PodcastID)
	c.JSON(http.StatusCreated, gin.H{"upload_id": uploadID, "status": "started"})
}

func finalizeUploadSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := userID.(string)

	var req struct {
		UploadID string `json:"upload_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upload ID is required"})
		return
	}

	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	var session *RecordingSession
	for _, s := range recordingSessions {
		if s.SessionID == req.UploadID && s.UserID == userIDStr {
			session = s
			break
		}
	}

	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Upload session not found"})
		return
	}

	session.IsComplete = true
	notifyRecordingComplete(session)

	ipAddress, userAgent := getClientInfo(c)
	monitoring.Logger.LogUploadSessionFinalized(userIDStr, session.PodcastID, req.UploadID, len(session.Chunks), ipAddress, userAgent)

	log.Printf("Upload session finalized: %s for user %s, podcast %s with %d chunks", 
		req.UploadID, userIDStr, session.PodcastID, len(session.Chunks))

	c.JSON(http.StatusOK, gin.H{"status": "finalized", "chunk_count": len(session.Chunks)})
}

func getUploadStatus(c *gin.Context) {
	uploadID := c.Param("uploadId")
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upload ID is required"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr := userID.(string)

	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	var session *RecordingSession
	for _, s := range recordingSessions {
		if s.SessionID == uploadID && s.UserID == userIDStr {
			session = s
			break
		}
	}

	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Upload session not found"})
		return
	}

	status := "started"
	if session.IsComplete {
		status = "completed"
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id":    session.SessionID,
		"user_id":      session.UserID,
		"podcast_id":   session.PodcastID,
		"recording_id": session.RecordingID,
		"status":       status,
		"chunk_count":  len(session.Chunks),
		"started_at":   session.StartTime,
		"is_complete":  session.IsComplete,
	})
}

func revokeUploadSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := userID.(string)

	var req struct {
		PodcastID string `json:"podcast_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "podcast_id is required"})
		return
	}

	err := revokeSession(userIDStr, req.PodcastID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ipAddress, userAgent := getClientInfo(c)
	monitoring.Logger.LogUploadSessionRevoked(userIDStr, req.PodcastID, "", ipAddress, userAgent)

	log.Printf("Upload session revoked for user %s, podcast %s", userIDStr, req.PodcastID)
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// Helper function to get client IP and User-Agent from Gin context
func getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.GetHeader("User-Agent")
	return
}