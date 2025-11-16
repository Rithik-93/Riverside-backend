package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lakeside/services/upload-service/internal/service"
	"github.com/lakeside/services/upload-service/monitoring"
	"github.com/lakeside/services/upload-service/pkg/types"
	"github.com/redis/go-redis/v9"
)

type UploadHandler struct {
	s3Client       *s3.Client
	bucket         string
	redisClient    *redis.Client
	sessionManager *service.SessionManager
}

func NewUploadHandler(s3Client *s3.Client, bucket string, redisClient *redis.Client, sessionManager *service.SessionManager) *UploadHandler {
	return &UploadHandler{
		s3Client:       s3Client,
		bucket:         bucket,
		redisClient:    redisClient,
		sessionManager: sessionManager,
	}
}

func (h *UploadHandler) GetPreSignedURL(c *gin.Context) {
	var req types.PreSignedURLRequest
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

	if req.PodcastID == "" && req.RecordingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "podcast_id or recording_id is required"})
		return
	}

	if !service.ValidateUserPermissions(h.redisClient, userID, req.PodcastID, req.RecordingID, req.IsFinal) {
		ipAddress, userAgent := getClientInfo(c)
		monitoring.Logger.LogSessionValidationFailed(userID, req.PodcastID, "No permission for this podcast/recording", ipAddress, userAgent)
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission for this podcast/recording"})
		return
	}

	actualChunkIndex := req.ChunkIndex
	if req.ChunkIndex == 0 && req.UserID != "" && req.PodcastID != "" {
		if session, exists := h.sessionManager.GetSession(req.UserID, req.PodcastID); exists {
			actualChunkIndex = len(session.Chunks)
			log.Printf("🔧 TEMP FIX: Using session chunk count %d instead of 0 (filename: %s)", actualChunkIndex, req.FileName)
		}
	}
	
	fileName := req.FileName
	if actualChunkIndex != req.ChunkIndex && strings.HasPrefix(req.FileName, "chunk_") {
		parts := strings.Split(req.FileName, "_")
		if len(parts) >= 3 {
			timestamp := parts[len(parts)-1]
			if strings.HasSuffix(timestamp, ".webm") {
				fileName = fmt.Sprintf("chunk_%d_%s", actualChunkIndex, timestamp)
			} else {
				fileName = fmt.Sprintf("chunk_%d_%s.webm", actualChunkIndex, timestamp)
			}
			log.Printf("Fixed filename: %s -> %s (chunk index corrected from %d to %d)", req.FileName, fileName, req.ChunkIndex, actualChunkIndex)
		}
	}
	
	s3Key := generateS3Key(fileName, userID, req.PodcastID, req.RecordingID, req.IsChunk)
	putObjectInput := &s3.PutObjectInput{
		Bucket: aws.String(h.bucket),
		Key:    aws.String(s3Key),
	}

	expirationTime := 3 * time.Minute
	presignClient := s3.NewPresignClient(h.s3Client)
	preSignedURL, err := presignClient.PresignPutObject(context.TODO(), putObjectInput, func(opts *s3.PresignOptions) {
		opts.Expires = expirationTime
	})
	if err != nil {
		log.Printf("Failed to generate pre-signed URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate pre-signed URL"})
		return
	}

	if req.UserID != "" && req.PodcastID != "" && req.RecordingID != "" {
		h.sessionManager.TrackChunk(req.UserID, req.PodcastID, req.RecordingID, s3Key, fileName, req.Timestamp, req.IsFinal, actualChunkIndex)
	}

	response := types.PreSignedURLResponse{
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

func (h *UploadHandler) StartUploadSession(c *gin.Context) {
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

	if !service.ValidateUserPermissions(h.redisClient, userIDStr, req.PodcastID, req.RecordingID, false) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission for this podcast/recording"})
		return
	}

	uploadID := uuid.New().String()
	
	if err := h.sessionManager.CreateSession(userIDStr, req.PodcastID, req.RecordingID, uploadID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	ipAddress, userAgent := getClientInfo(c)
	monitoring.Logger.LogUploadSessionStarted(userIDStr, req.PodcastID, uploadID, ipAddress, userAgent)

	log.Printf("Upload session started: %s for user %s, podcast %s", uploadID, userIDStr, req.PodcastID)
	c.JSON(http.StatusCreated, gin.H{"upload_id": uploadID, "status": "started"})
}

func (h *UploadHandler) FinalizeUploadSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := userID.(string)

	var req struct {
		UploadID string `json:"upload_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upload ID is required"})
		return
	}

	session, exists := h.sessionManager.GetSessionByID(req.UploadID, userIDStr)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Upload session not found"})
		return
	}

	ipAddress, userAgent := getClientInfo(c)
	monitoring.Logger.LogUploadSessionFinalized(userIDStr, session.PodcastID, req.UploadID, len(session.Chunks), ipAddress, userAgent)

	log.Printf("Upload session finalized: %s for user %s, podcast %s with %d chunks", 
		req.UploadID, userIDStr, session.PodcastID, len(session.Chunks))

	c.JSON(http.StatusOK, gin.H{"status": "finalized", "chunk_count": len(session.Chunks)})
}

func (h *UploadHandler) GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("uploadId")
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upload ID is required"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr := userID.(string)

	session, exists := h.sessionManager.GetSessionByID(uploadID, userIDStr)
	if !exists {
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

func (h *UploadHandler) RevokeUploadSession(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userIDStr := userID.(string)

	var req struct {
		PodcastID string `json:"podcast_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "podcast_id is required"})
		return
	}

	err := h.sessionManager.RevokeSession(userIDStr, req.PodcastID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ipAddress, userAgent := getClientInfo(c)
	monitoring.Logger.LogUploadSessionRevoked(userIDStr, req.PodcastID, "", ipAddress, userAgent)

	log.Printf("Upload session revoked for user %s, podcast %s", userIDStr, req.PodcastID)
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

func (h *UploadHandler) GetDownloadURL(c *gin.Context) {
	var req struct {
		S3URL string `json:"s3_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "s3_url is required"})
		return
	}

	s3Key := req.S3URL
	if strings.HasPrefix(s3Key, "https://") {
		idx := strings.Index(s3Key[8:], "/")
		if idx != -1 {
			s3Key = s3Key[8+idx+1:]
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid S3 URL format"})
			return
		}
	} else if strings.HasPrefix(s3Key, "s3://") {
		parts := strings.SplitN(s3Key[5:], "/", 2)
		if len(parts) >= 2 {
			s3Key = parts[1]
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid S3 URL format"})
			return
		}
	}

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(h.bucket),
		Key:    aws.String(s3Key),
	}

	expirationTime := 15 * time.Minute
	presignClient := s3.NewPresignClient(h.s3Client)
	preSignedURL, err := presignClient.PresignGetObject(context.TODO(), getObjectInput, func(opts *s3.PresignOptions) {
		opts.Expires = expirationTime
	})
	if err != nil {
		log.Printf("Failed to generate download URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate download URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"download_url": preSignedURL.URL,
		"expires_in":   int(expirationTime.Seconds()),
	})
}

func validatePresignedURLRequest(req *types.PreSignedURLRequest) error {
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
	
	if req.FileSize > 0 && req.FileSize > 100*1024*1024 {
		return fmt.Errorf("file size too large: %d bytes", req.FileSize)
	}
	
	if req.ChunkIndex < 0 {
		return fmt.Errorf("chunk index must be non-negative")
	}
	
	return nil
}

func generateS3Key(fileName, userID, podcastID, recordingID string, isChunk bool) string {
	if userID != "" && podcastID != "" && recordingID != "" {
		return fmt.Sprintf("uploads/recordings/%s/%s/%s/chunks/%s", 
			podcastID, recordingID, userID, fileName)
	} else {
		uuid := uuid.New().String()
		return fmt.Sprintf("uploads/files/%s/%s", uuid, fileName)
	}
}

func getClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	ipAddress = c.ClientIP()
	userAgent = c.GetHeader("User-Agent")
	return
}

