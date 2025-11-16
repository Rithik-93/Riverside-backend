package service

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"video-processor/internal/infrastructure"
	"video-processor/pkg/types"

	"gorm.io/gorm"
)

type VideoProcessor struct {
	s3Client            *infrastructure.S3Client
	db                  *gorm.DB
	processedRecordings map[string]bool
	mutex               sync.RWMutex
}

func NewVideoProcessor(s3Client *infrastructure.S3Client, db *gorm.DB) *VideoProcessor {
	return &VideoProcessor{
		s3Client:            s3Client,
		db:                  db,
		processedRecordings: make(map[string]bool),
	}
}

func (vp *VideoProcessor) ProcessRecording(data *types.RecordingCompleteData, userID, roomID string) error {
	// Check for duplicate processing
	processingKey := fmt.Sprintf("%s_%s", data.SessionID, userID)
	vp.mutex.Lock()
	if vp.processedRecordings[processingKey] {
		vp.mutex.Unlock()
		log.Printf("⚠️ Recording %s for user %s already processed, skipping duplicate", data.SessionID, userID)
		return nil
	}
	vp.processedRecordings[processingKey] = true
	vp.mutex.Unlock()

	log.Printf("🎬 Processing recording: %s", data.SessionID)
	log.Printf("   - User: %s, Room: %s", userID, roomID)
	log.Printf("   - Chunks to concatenate: %d", data.TotalChunks)
	log.Printf("   - Duration: %.2f seconds", data.Duration)

	// Download and concatenate chunks
	if err := vp.concatenateVideoChunks(data, userID, roomID); err != nil {
		log.Printf("❌ Failed to process recording %s: %v", data.SessionID, err)
		// Remove from processed list on failure so it can be retried
		vp.mutex.Lock()
		delete(vp.processedRecordings, processingKey)
		vp.mutex.Unlock()
		return err
	}

	log.Printf("✅ Successfully processed recording: %s", data.SessionID)
	return nil
}

func (vp *VideoProcessor) concatenateVideoChunks(data *types.RecordingCompleteData, userID, roomID string) error {

	/*
	Fetch s3 list until the final chunk is uploaded
	because final chunks might be uploading
	*/
	var s3Keys []string
	var err error
	maxRetries := 5
	retryDelay := 2 * time.Second
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		s3Keys, err = vp.s3Client.ListChunks(data.S3Bucket, data.ChunkFolder)
		if err != nil {
			return fmt.Errorf("failed to list chunks from S3: %v", err)
		}

		if len(s3Keys) == 0 {
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("no chunks found in S3 folder after %d attempts: %s", maxRetries, data.ChunkFolder)
		}

		if len(s3Keys) == data.TotalChunks {
			break
		}

		if attempt < maxRetries {
			time.Sleep(retryDelay)
		}
	}
	

	// Create temporary directory for processing
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("video_processing_%s", data.SessionID))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)


	// Download all chunks with retry logic
	var chunkFiles []string
	var totalSize int64
	for i, s3Key := range s3Keys {
		chunkFile := filepath.Join(tempDir, fmt.Sprintf("chunk_%03d.webm", i))

		// Retry download up to 3 times
		var downloadErr error
		for attempt := 1; attempt <= 3; attempt++ {
			if err := vp.s3Client.DownloadChunk(data.S3Bucket, s3Key, chunkFile); err != nil {
				downloadErr = err
				log.Printf("⚠️ Download attempt %d failed for chunk %s: %v", attempt, s3Key, err)
				if attempt < 3 {
					time.Sleep(time.Duration(attempt) * time.Second)
				}
			} else {
				downloadErr = nil
				break
			}
		}

		if downloadErr != nil {
			return fmt.Errorf("failed to download chunk %s after 3 attempts: %v", s3Key, downloadErr)
		}

		fileInfo, err := os.Stat(chunkFile)
		if err != nil {
			return fmt.Errorf("failed to stat downloaded chunk %s: %v", chunkFile, err)
		}
		
		chunkSize := fileInfo.Size()
		if chunkSize == 0 {
			return fmt.Errorf("downloaded chunk %s is empty", s3Key)
		}
		
		totalSize += chunkSize
		chunkFiles = append(chunkFiles, chunkFile)
	}
	

	// Concatenate chunks using simple file concatenation
	finalVideoPath := filepath.Join(tempDir, "final_video.webm")
	if err := concatenateFiles(chunkFiles, finalVideoPath); err != nil {
		return fmt.Errorf("failed to concatenate chunks: %v", err)
	}

	log.Printf("🔗 Concatenated %d chunks into: %s", len(chunkFiles), finalVideoPath)

	// Upload final video back to S3
	if err := vp.s3Client.UploadFinalVideo(data.S3Bucket, data.OutputPath, finalVideoPath); err != nil {
		return fmt.Errorf("failed to upload final video: %v", err)
	}

	log.Printf("📤 Uploaded final video to: %s", data.OutputPath)

	if err := vp.saveRecordingToDB(data, userID); err != nil {
		log.Printf("Failed to save recording to database: %v", err)
	}

	if os.Getenv("CLEANUP_CHUNKS") == "true" {
		go vp.s3Client.CleanupChunks(data.S3Bucket, s3Keys)
	}

	return nil
}

func (vp *VideoProcessor) saveRecordingToDB(data *types.RecordingCompleteData, userID string) error {
	if vp.db == nil {
		return fmt.Errorf("database connection not available")
	}

	var s3URL string
	if data.S3Endpoint != "" {
		s3URL = fmt.Sprintf("%s/%s/%s", data.S3Endpoint, data.S3Bucket, data.OutputPath)
	} else {
		s3URL = fmt.Sprintf("s3://%s/%s", data.S3Bucket, data.OutputPath)
	}

	durationMs := int64(data.Duration * 1000)
	startedAt := time.Unix(data.StartTime, 0)
	endedAt := time.Unix(data.EndTime, 0)

	var existingRecording infrastructure.Recording
	var podID uint64
	err := vp.db.Where("recording_id = ?", data.RecordingID).First(&existingRecording).Error
	if err == nil {
		podID = existingRecording.PodID
	} else {
		type Pod struct {
			ID uint64 `gorm:"column:id"`
		}
		var pod Pod
		
		err = vp.db.Table("pods").
			Where("host_user_id = ? AND is_active = ?", userID, true).
			Order("created_at DESC").
			First(&pod).Error
		
		if err != nil {
			err = vp.db.Table("pods").
				Joins("INNER JOIN pod_participants ON pods.id = pod_participants.pod_id").
				Where("pod_participants.user_id = ? AND pods.is_active = ?", userID, true).
				Order("pods.created_at DESC").
				First(&pod).Error
		}
		
		if err != nil {
			return fmt.Errorf("cannot find pod for user %s: %v", userID, err)
		}
		
		podID = pod.ID
		log.Printf("Found pod_id=%d for user %s", podID, userID)
	}

	recording := infrastructure.Recording{
		PodID:       podID,
		RecordingID: data.RecordingID,
		S3URL:       &s3URL,
		DurationMs:  &durationMs,
		State:       "completed",
		StartedAt:   startedAt,
		EndedAt:     &endedAt,
	}
	
	if err := vp.db.Where("recording_id = ?", data.RecordingID).
		Assign(recording).
		FirstOrCreate(&recording).Error; err != nil {
		return fmt.Errorf("failed to save recording: %v", err)
	}

	userLink := infrastructure.UserRecordingLink{
		UserID:      userID,
		RecordingID: data.RecordingID,
		S3URL:       s3URL,
	}
	if err := vp.db.Where("user_id = ? AND recording_id = ?", userID, data.RecordingID).
		Assign(userLink).
		FirstOrCreate(&userLink).Error; err != nil {
		return fmt.Errorf("failed to save user recording link: %v", err)
	}

	log.Printf("✅ Saved recording: RecordingID=%s, PodID=%d, UserID=%s, S3URL=%s", data.RecordingID, podID, userID, s3URL)
	return nil
}

func concatenateFiles(chunkFiles []string, outputPath string) error {
	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()

	// Simple concatenation for WebM files
	for i, chunkFile := range chunkFiles {
		chunk, err := os.Open(chunkFile)
		if err != nil {
			return fmt.Errorf("failed to open chunk %s: %v", chunkFile, err)
		}

		if i == 0 {
			// Copy the entire first chunk (includes WebM header)
			_, err = io.Copy(output, chunk)
		} else {
			// For subsequent chunks, concatenate
			_, err = io.Copy(output, chunk)
		}

		chunk.Close()

		if err != nil {
			return fmt.Errorf("failed to copy chunk %s: %v", chunkFile, err)
		}
	}

	return nil
}

// func extractChunkIndexFromKey(s3Key string) int {
// 	parts := bytes.Split([]byte(s3Key), []byte("chunk_"))
// 	if len(parts) < 2 {
// 		return -1
// 	}

// 	chunkPart := parts[1]
// 	underscoreIndex := bytes.Index(chunkPart, []byte("_"))
// 	if underscoreIndex == -1 {
// 		return -1
// 	}

// 	chunkNum := chunkPart[:underscoreIndex]
// 	var result int
// 	for _, b := range chunkNum {
// 		if b >= '0' && b <= '9' {
// 			result = result*10 + int(b-'0')
// 		} else {
// 			break
// 		}
// 	}
// 	return result
// }

