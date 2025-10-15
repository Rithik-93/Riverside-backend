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
)

type VideoProcessor struct {
	s3Client            *infrastructure.S3Client
	processedRecordings map[string]bool
	mutex               sync.RWMutex
}

func NewVideoProcessor(s3Client *infrastructure.S3Client) *VideoProcessor {
	return &VideoProcessor{
		s3Client:            s3Client,
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
	// List objects from S3
	s3Keys, err := vp.s3Client.ListChunks(data.S3Bucket, data.ChunkFolder)
	if err != nil {
		return fmt.Errorf("failed to list chunks from S3: %v", err)
	}

	if len(s3Keys) == 0 {
		return fmt.Errorf("no chunks found in S3 folder: %s", data.ChunkFolder)
	}

	log.Printf("✅ Processing %d chunks found in S3 folder", len(s3Keys))

	// Create temporary directory for processing
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("video_processing_%s", data.SessionID))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	log.Printf("📁 Created temporary directory: %s", tempDir)

	// Download all chunks with retry logic
	var chunkFiles []string
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
	if err := vp.s3Client.UploadFinalVideo(data.S3Bucket, data.OutputPath, finalVideoPath); err != nil {
		return fmt.Errorf("failed to upload final video: %v", err)
	}

	log.Printf("📤 Uploaded final video to: %s", data.OutputPath)

	// Optionally clean up individual chunks from S3
	if os.Getenv("CLEANUP_CHUNKS") == "true" {
		go vp.s3Client.CleanupChunks(data.S3Bucket, s3Keys)
	}

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

