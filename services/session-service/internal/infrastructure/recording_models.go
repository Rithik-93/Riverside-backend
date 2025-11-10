package infrastructure

import (
	"time"
)

type Recording struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	PodID       uint64     `gorm:"index;not null" json:"pod_id"`
	RecordingID string     `gorm:"uniqueIndex;not null" json:"recording_id"`
	S3URL       *string    `gorm:"type:varchar(1000)" json:"s3_url"`
	DurationMs  *int64     `gorm:"default:0" json:"duration_ms"`
	State       string     `gorm:"index;default:'started'" json:"state"`
	StartedAt   time.Time  `gorm:"not null;default:now()" json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (Recording) TableName() string {
	return "recordings"
}

type UserRecordingLink struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      string    `gorm:"index;not null" json:"user_id"`
	RecordingID string    `gorm:"index;not null" json:"recording_id"`
	S3URL       string    `gorm:"type:varchar(1000);not null" json:"s3_url"`
	FileSize    *int64    `json:"file_size"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (UserRecordingLink) TableName() string {
	return "user_recording_links"
}

