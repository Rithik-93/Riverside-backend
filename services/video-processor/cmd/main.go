package main

import (
	"log"
	"time"

	"video-processor/internal/infrastructure"
	"video-processor/internal/service"
	"video-processor/pkg/types"

	"github.com/joho/godotenv"
	eventspb "github.com/lakeside/backend/protos/gen/events"
	"google.golang.org/protobuf/proto"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	redisConsumer := infrastructure.NewRedisConsumer()
	s3Client := infrastructure.NewS3Client()
	db := infrastructure.ConnectDB()
	videoProcessor := service.NewVideoProcessor(s3Client, db)

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
		eventType := protoEvent.GetEventType()
		log.Printf("🔍 Received event type: %s", eventType)
		if eventType == "recording_complete" {
			go processRecordingCompletionProto(protoEvent, videoProcessor)
		} else {
		}
	}
}

func processRecordingCompletionProto(protoEvent *eventspb.RedisEvent, videoProcessor *service.VideoProcessor) {
	recordingData := protoEvent.GetRecordingComplete()
	if recordingData == nil {
		log.Printf("Failed to get recording complete data from protobuf event")
		return
	}

	log.Printf("Recording data extracted: SessionID=%s, RecordingID=%s, TotalChunks=%d", 
		recordingData.GetSessionId(), recordingData.GetRecordingId(), recordingData.GetTotalChunks())

	// Convert protobuf data to types.RecordingCompleteData
	data := types.RecordingCompleteData{
		RecordingID: recordingData.GetRecordingId(),
		PodcastID:   recordingData.GetPodcastId(),
		SessionID:   recordingData.GetSessionId(),
		TotalChunks: int(recordingData.GetTotalChunks()),
		S3Bucket:    recordingData.GetS3Bucket(),
		S3Region:    recordingData.GetS3Region(),
		S3Endpoint:  recordingData.GetS3Endpoint(),
		StartTime:   recordingData.GetStartTime(),
		EndTime:     recordingData.GetEndTime(),
		Duration:    recordingData.GetDuration(),
		OutputPath:  recordingData.GetOutputPath(),
		ContentType: recordingData.GetContentType(),
		ChunkFolder: recordingData.GetChunkFolder(),
	}

	videoProcessor.ProcessRecording(&data, protoEvent.GetUserId(), protoEvent.GetRoomId())
}
