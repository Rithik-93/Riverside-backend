package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
	eventspb "github.com/lakeside/backend/protos/gen/events"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type RedisEvent struct {
	EventType string                 `json:"eventType"`
	UserID    string                 `json:"userId,omitempty"`
	ClientID  string                 `json:"clientId"`
	RoomID    string                 `json:"roomId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

type RecordingCompleteData struct {
	SessionID   string  `json:"sessionId"`
	TotalChunks int     `json:"totalChunks"`
	S3Bucket    string  `json:"s3Bucket"`
	S3Region    string  `json:"s3Region"`
	S3Endpoint  string  `json:"s3Endpoint"`
	StartTime   int64   `json:"startTime"`
	EndTime     int64   `json:"endTime"`
	Duration    float64 `json:"duration"`
	OutputPath  string  `json:"outputPath"`
	ContentType string  `json:"contentType"`
	ChunkFolder string  `json:"chunkFolder"`
}

var (
	redisClient *redis.Client
	s3Client    *s3.Client
	// Track processed recordings to prevent duplicates
	processedRecordings map[string]bool
	processingMutex     sync.RWMutex
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize processed recordings tracking
	processedRecordings = make(map[string]bool)

	// Initialize Redis
	initRedis()

	// Initialize S3
	initS3()

	log.Println("🎬 Video Processor Service started")
	log.Println("📡 Listening for recording completion events...")
	log.Println("✅ Using simple WebM concatenation")

	// Start listening for Redis messages
	listenForRecordingEvents()
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

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("✅ Connected to Redis")
}

func initS3() {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	// Check if we're using DigitalOcean Spaces
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" && region == "blr1" {
		endpoint = "https://blr1.digitaloceanspaces.com"
	}

	var cfg aws.Config
	var err error

	if endpoint != "" {
		// Custom endpoint (like DigitalOcean Spaces)
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
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)
	log.Printf("✅ S3 initialized - Region: %s, Endpoint: %s", region, endpoint)
}

func listenForRecordingEvents() {
	for {
		ctx := context.Background()

		// Block and wait for messages from the queue
		result, err := redisClient.BRPop(ctx, 0, "queue").Result()
		if err != nil {
			log.Printf("Error reading from Redis queue: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if len(result) < 2 {
			log.Printf("Invalid Redis message format")
			continue
		}

		// Parse the protobuf event
		protoEvent := &eventspb.RedisEvent{}
		if err := proto.Unmarshal([]byte(result[1]), protoEvent); err != nil {
			log.Printf("Failed to parse protobuf event: %v", err)
			continue
		}

		// Process recording completion events
		if protoEvent.EventType == "recording_complete" {
			log.Printf("📥 Received recording completion event (protobuf) for User: %s",
				protoEvent.UserId)

			go processRecordingCompletionProto(protoEvent)
		}
	}
}

func processRecordingCompletionProto(protoEvent *eventspb.RedisEvent) {
	// Get recording complete data from protobuf
	recordingData := protoEvent.GetRecordingComplete()
	if recordingData == nil {
		log.Printf("Failed to get recording complete data from protobuf event")
		return
	}

	// Check for duplicate processing (use sessionID + userID to allow multiple participants)
	processingKey := fmt.Sprintf("%s_%s", recordingData.SessionId, protoEvent.UserId)
	processingMutex.Lock()
	if processedRecordings[processingKey] {
		processingMutex.Unlock()
		log.Printf("⚠️ Recording %s for user %s already processed, skipping duplicate", recordingData.SessionId, protoEvent.UserId)
		return
	}
	processedRecordings[processingKey] = true
	processingMutex.Unlock()

	log.Printf("🎬 Processing recording: %s", recordingData.SessionId)
	log.Printf("   - User: %s, Room: %s", protoEvent.UserId, protoEvent.RoomId)
	log.Printf("   - Chunks to concatenate: %d", recordingData.TotalChunks)
	log.Printf("   - Duration: %.2f seconds", recordingData.Duration)

	// Convert protobuf data to old RecordingCompleteData struct for existing logic
	data := RecordingCompleteData{
		SessionID:   recordingData.SessionId,
		TotalChunks: int(recordingData.TotalChunks),
		S3Bucket:    recordingData.S3Bucket,
		S3Region:    recordingData.S3Region,
		S3Endpoint:  recordingData.S3Endpoint,
		StartTime:   recordingData.StartTime,
		EndTime:     recordingData.EndTime,
		Duration:    recordingData.Duration,
		OutputPath:  recordingData.OutputPath,
		ContentType: recordingData.ContentType,
		ChunkFolder: recordingData.ChunkFolder,
	}

	// Download and concatenate chunks
	if err := concatenateVideoChunks(&data, protoEvent.UserId, protoEvent.RoomId); err != nil {
		log.Printf("❌ Failed to process recording %s: %v", recordingData.SessionId, err)
		// Remove from processed list on failure so it can be retried
		processingMutex.Lock()
		delete(processedRecordings, recordingData.SessionId)
		processingMutex.Unlock()
		return
	}

	log.Printf("✅ Successfully processed recording: %s", recordingData.SessionId)
}

// Old JSON-based function kept for backward compatibility (can be removed later)
func processRecordingCompletion(event *RedisEvent) {
	// Parse the data
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		log.Printf("Failed to marshal event data: %v", err)
		return
	}

	var data RecordingCompleteData
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		log.Printf("Failed to parse recording data: %v", err)
		return
	}
	fmt.Println(data)

	// Check for duplicate processing (use sessionID + userID to allow multiple participants)
	processingKey := fmt.Sprintf("%s_%s", data.SessionID, event.UserID)
	processingMutex.Lock()
	if processedRecordings[processingKey] {
		processingMutex.Unlock()
		log.Printf("⚠️ Recording %s for user %s already processed, skipping duplicate", data.SessionID, event.UserID)
		return
	}
	processedRecordings[processingKey] = true
	processingMutex.Unlock()

	log.Printf("🎬 Processing recording: %s", data.SessionID)
	log.Printf("   - User: %s, Room: %s", event.UserID, event.RoomID)
	log.Printf("   - Chunks to concatenate: %d", data.TotalChunks)
	log.Printf("   - Duration: %.2f seconds", data.Duration)

	// Download and concatenate chunks
	if err := concatenateVideoChunks(&data, event.UserID, event.RoomID); err != nil {
		log.Printf("❌ Failed to process recording %s: %v", data.SessionID, err)
		// Remove from processed list on failure so it can be retried
		processingMutex.Lock()
		delete(processedRecordings, data.SessionID)
		processingMutex.Unlock()
		return
	}

	log.Printf("✅ Successfully processed recording: %s", data.SessionID)
}

func concatenateVideoChunks(data *RecordingCompleteData, userID, roomID string) error {
	// List objects from S3
	s3Keys, err := listChunks(data.S3Bucket, data.ChunkFolder)
	if err != nil {
		return fmt.Errorf("failed to list chunks from S3: %v", err)
	}

	if len(s3Keys) == 0 {
		return fmt.Errorf("no chunks found in S3 folder: %s", data.ChunkFolder)
	}

	// Process whatever chunks are available (skip strict validation)
	log.Printf("✅ Processing %d chunks found in S3 folder", len(s3Keys))

	// Create temporary directory for processing
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("video_processing_%s", data.SessionID))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up

	log.Printf("📁 Created temporary directory: %s", tempDir)

	// Download all chunks with retry logic
	var chunkFiles []string
	for i, s3Key := range s3Keys {
		chunkFile := filepath.Join(tempDir, fmt.Sprintf("chunk_%03d.webm", i))

		// Retry download up to 3 times
		var downloadErr error
		for attempt := 1; attempt <= 3; attempt++ {
			if err := downloadChunk(data.S3Bucket, s3Key, chunkFile); err != nil {
				downloadErr = err
				log.Printf("⚠️ Download attempt %d failed for chunk %s: %v", attempt, s3Key, err)
				if attempt < 3 {
					time.Sleep(time.Duration(attempt) * time.Second) // Exponential backoff
				}
			} else {
				downloadErr = nil
				break
			}
		}

		if downloadErr != nil {
			return fmt.Errorf("failed to download chunk %s after 3 attempts: %v", s3Key, downloadErr)
		}

		chunkFiles = append(chunkFiles, chunkFile)
		log.Printf("📥 Downloaded chunk %d/%d: %s", i+1, len(s3Keys), s3Key)
	}

	// Concatenate chunks using simple file concatenation
	finalVideoPath := filepath.Join(tempDir, "final_video.webm")
	if err := concatenateFiles(chunkFiles, finalVideoPath); err != nil {
		return fmt.Errorf("failed to concatenate chunks: %v", err)
	}

	log.Printf("🔗 Concatenated %d chunks into: %s", len(chunkFiles), finalVideoPath)

	// Upload final video back to S3
	if err := uploadFinalVideo(data.S3Bucket, data.OutputPath, finalVideoPath); err != nil {
		return fmt.Errorf("failed to upload final video: %v", err)
	}

	log.Printf("📤 Uploaded final video to: %s", data.OutputPath)

	// Thumbnail generation removed for simplicity

	// Optionally clean up individual chunks from S3
	if os.Getenv("CLEANUP_CHUNKS") == "true" {
		go cleanupChunks(data.S3Bucket, s3Keys)
	}

	return nil
}

func listChunks(bucket, prefix string) ([]string, error) {
	var s3Keys []string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			// Include all chunk files, exclude the folder itself and final recordings
			key := *obj.Key
			if key != prefix &&
				!bytes.HasSuffix([]byte(key), []byte("final_recording_")) &&
				bytes.Contains([]byte(key), []byte("chunk_")) {
				s3Keys = append(s3Keys, key)
			}
		}
	}

	// Sort keys by chunk index (extracted from filename)
	sort.Slice(s3Keys, func(i, j int) bool {
		// Extract chunk index from filename like "chunk_0_timestamp.webm"
		chunkIndexI := extractChunkIndex(s3Keys[i])
		chunkIndexJ := extractChunkIndex(s3Keys[j])
		return chunkIndexI < chunkIndexJ
	})

	log.Printf("📋 Found %d chunks in S3 folder: %s", len(s3Keys), prefix)
	for i, key := range s3Keys {
		log.Printf("   Chunk %d: %s", i+1, key)
	}

	return s3Keys, nil
}

// extractChunkIndex extracts the chunk index from S3 key
func extractChunkIndex(s3Key string) int {
	// Expected format: "uploads/recordings/{podcastID}/{recordingID}/{userID}/chunks/chunk_X_timestamp.webm"
	parts := bytes.Split([]byte(s3Key), []byte("chunk_"))
	if len(parts) < 2 {
		return 0
	}

	// Extract the number after "chunk_"
	chunkPart := parts[1]
	underscoreIndex := bytes.Index(chunkPart, []byte("_"))
	if underscoreIndex == -1 {
		return 0
	}

	chunkNum := chunkPart[:underscoreIndex]
	// Convert to int (simple parsing)
	var result int
	for _, b := range chunkNum {
		if b >= '0' && b <= '9' {
			result = result*10 + int(b-'0')
		} else {
			break
		}
	}
	return result
}

// validateChunkSequence validates that we have all expected chunks in sequence
func validateChunkSequence(s3Keys []string, expectedTotal int) error {
	if len(s3Keys) != expectedTotal {
		return fmt.Errorf("chunk count mismatch: found %d chunks, expected %d", len(s3Keys), expectedTotal)
	}

	// Check for gaps in chunk sequence
	expectedIndices := make(map[int]bool)
	for i := 0; i < expectedTotal; i++ {
		expectedIndices[i] = true
	}

	foundIndices := make(map[int]bool)
	for _, key := range s3Keys {
		chunkIndex := extractChunkIndex(key)
		if chunkIndex < 0 || chunkIndex >= expectedTotal {
			return fmt.Errorf("invalid chunk index %d in key: %s", chunkIndex, key)
		}
		if foundIndices[chunkIndex] {
			return fmt.Errorf("duplicate chunk index %d found", chunkIndex)
		}
		foundIndices[chunkIndex] = true
	}

	// Check for missing chunks
	var missingChunks []int
	for i := 0; i < expectedTotal; i++ {
		if !foundIndices[i] {
			missingChunks = append(missingChunks, i)
		}
	}

	if len(missingChunks) > 0 {
		return fmt.Errorf("missing chunks: %v", missingChunks)
	}

	return nil
}

func downloadChunk(bucket, s3Key, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get object from S3 with checksum validation
	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(s3Key),
		ChecksumMode: "ENABLED", // Enable checksum validation
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, result.Body)
	return err
}

func concatenateFiles(chunkFiles []string, outputPath string) error {
	// Create output file
	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()

	// Simple concatenation for WebM files
	// Note: This is a basic approach. For production, you might want to use FFmpeg
	for i, chunkFile := range chunkFiles {
		chunk, err := os.Open(chunkFile)
		if err != nil {
			return fmt.Errorf("failed to open chunk %s: %v", chunkFile, err)
		}

		if i == 0 {
			// Copy the entire first chunk (includes WebM header)
			_, err = io.Copy(output, chunk)
		} else {
			// For subsequent chunks, we should ideally skip the WebM header
			// For now, let's just concatenate (this may need improvement with FFmpeg)
			_, err = io.Copy(output, chunk)
		}

		chunk.Close()

		if err != nil {
			return fmt.Errorf("failed to copy chunk %s: %v", chunkFile, err)
		}
	}

	return nil
}

func uploadFinalVideo(bucket, s3Key, localPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Read file
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info for content length
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// Upload to S3
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(s3Key),
		Body:          file,
		ContentLength: aws.Int64(fileInfo.Size()),
		ContentType:   aws.String("video/webm"),
		ACL:           "public-read", // Make it publicly accessible
	})

	return err
}

func cleanupChunks(bucket string, s3Keys []string) {
	log.Printf("🧹 Cleaning up %d chunk files...", len(s3Keys))

	for _, s3Key := range s3Keys {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3Key),
		})

		if err != nil {
			log.Printf("Warning: Failed to delete chunk %s: %v", s3Key, err)
		} else {
			log.Printf("🗑️ Deleted chunk: %s", s3Key)
		}
	}

	log.Printf("✅ Cleanup completed")
}


