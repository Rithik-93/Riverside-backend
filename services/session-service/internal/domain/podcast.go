package domain

import (
	"time"
)

type Podcast struct {
	ID               int64      `gorm:"primaryKey;column:id" json:"id"`
	HostUserID       string     `gorm:"column:host_user_id;not null" json:"host_user_id"`
	TargetUserID     *string    `gorm:"column:target_user_id" json:"target_user_id,omitempty"`
	ParticipantCount int        `gorm:"column:participant_count;default:1" json:"participant_count"`
	Status           string     `gorm:"column:status;default:ONGOING" json:"status"`
	IsRecording      bool       `gorm:"column:is_recording;default:false" json:"is_recording"`
	StartedAt        time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	EndedAt          *time.Time `gorm:"column:ended_at" json:"ended_at,omitempty"`
	IsActive         bool       `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null;autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updated_at"`
}

func (Podcast) TableName() string {
	return "pods"
}

func NewPodcast(hostUserID string) *Podcast {
	now := time.Now()
	return &Podcast{
		HostUserID:       hostUserID,
		ParticipantCount: 1,
		Status:           "ONGOING",
		IsRecording:      false,
		StartedAt:        now,
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

