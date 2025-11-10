package types

type RedisEvent struct {
	EventType string                 `json:"eventType"`
	UserID    string                 `json:"userId,omitempty"`
	ClientID  string                 `json:"clientId"`
	RoomID    string                 `json:"roomId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

type RecordingCompleteData struct {
	RecordingID string  `json:"recordingId"`
	PodcastID   string  `json:"podcastId"`
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

