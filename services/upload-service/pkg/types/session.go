package types

import "time"

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

