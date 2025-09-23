package infrastructure

import (
	"time"
)

type Status string

const (
	StatusOngoing   Status = "ONGOING"
	StatusCompleted Status = "COMPLETED"
	StatusCancelled Status = "CANCELLED"
	StatusFailed    Status = "FAILED"
)


type Pod struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	HostUserID        string    `gorm:"index;not null" json:"host_user_id"`
	TargetUserID      *string   `gorm:"index" json:"target_user_id"`
	ParticipantCount  int       `gorm:"default:1" json:"participant_count"`
	Status            Status    `gorm:"index;default:ONGOING" json:"status"`
	IsRecording       bool      `gorm:"default:false" json:"is_recording"`
	StartedAt         time.Time `gorm:"not null;default:now()" json:"started_at"`
	EndedAt           *time.Time `json:"ended_at"`
	IsActive          bool      `gorm:"default:true" json:"is_active"`
	CreatedAt         time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (Pod) TableName() string {
	return "pods"
}

type PodParticipant struct {
	PodID    uint64 `gorm:"primaryKey;autoIncrement:false"`
	UserID   string `gorm:"primaryKey"`
	JoinedAt time.Time `gorm:"not null;default:now()"`
}

func (PodParticipant) TableName() string {
	return "pod_participants"
}


