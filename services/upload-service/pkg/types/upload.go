package types

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

