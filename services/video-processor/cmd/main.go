package main

import (
	"log"
	"time"

	"github.com/joho/godotenv"
	eventspb "github.com/lakeside/backend/protos/gen/events"
	"google.golang.org/protobuf/proto"
	"video-processor/internal/infrastructure"
	"video-processor/internal/service"
	"video-processor/pkg/types"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	redisConsumer := infrastructure.NewRedisConsumer()
	s3Client := infrastructure.NewS3Client()
	videoProcessor := service.NewVideoProcessor(s3Client)

	log.Println("🎬 Video Processor Service started")
	log.Println("📡 Listening for recording completion events...")
	log.Println("✅ Using simple WebM concatenation")

	listenForRecordingEvents(redisConsumer, videoProcessor)
}

func listenForRecordingEvents(redisConsumer *infrastructure.RedisConsumer, videoProcessor *service.VideoProcessor) {
	for {
		message, err := redisConsumer.ConsumeQueue("queue")
		if err != nil {
			log.Printf("Error reading from Redis queue: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if message == nil {
			log.Printf("Invalid Redis message format")
			continue
		}

		// Parse the protobuf event
		protoEvent := &eventspb.RedisEvent{}
		if err := proto.Unmarshal(message, protoEvent); err != nil {
			log.Printf("Failed to parse protobuf event: %v", err)
			continue
		}

		// Process recording completion events
		if protoEvent.EventType == "recording_complete" {
			log.Printf("📥 Received recording completion event (protobuf) for User: %s", protoEvent.UserId)
			go processRecordingCompletionProto(protoEvent, videoProcessor)
		}
	}
}

func processRecordingCompletionProto(protoEvent *eventspb.RedisEvent, videoProcessor *service.VideoProcessor) {
	recordingData := protoEvent.GetRecordingComplete()
	if recordingData == nil {
		log.Printf("Failed to get recording complete data from protobuf event")
		return
	}

	// Convert protobuf data to types.RecordingCompleteData
	data := types.RecordingCompleteData{
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

	videoProcessor.ProcessRecording(&data, protoEvent.UserId, protoEvent.RoomId)
}
