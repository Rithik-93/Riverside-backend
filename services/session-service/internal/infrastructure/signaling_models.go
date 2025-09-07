package infrastructure

import (
	"time"
)

type ClientConnection struct {
	ID             uint      `gorm:"primaryKey"`
	ClientID       string    `gorm:"uniqueIndex;not null"`
	UserID         string    `gorm:"index;not null"`
	ConnectedAt    time.Time `gorm:"not null"`
	DisconnectedAt *time.Time
	UserAgent      string `gorm:"size:500"`
	IPAddress      string `gorm:"size:45"`
	IsActive       bool   `gorm:"not null;default:true"`
	DurationMs     int64  `gorm:"default:0"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (ClientConnection) TableName() string {
	return "client_connections"
}

type RoomParticipation struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"index;not null"`
	ClientID  string    `gorm:"index;not null"`
	RoomID    string    `gorm:"index;not null"`
	JoinedAt  time.Time `gorm:"not null"`
	LeftAt    *time.Time
	IsActive  bool `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (RoomParticipation) TableName() string {
	return "room_participations"
}

type CallSession struct {
	ID           uint      `gorm:"primaryKey"`
	UserID       string    `gorm:"index;not null"`
	ClientID     string    `gorm:"index;not null"`
	RoomID       string    `gorm:"index;not null"`
	TargetUserID string    `gorm:"index"`
	StartedAt    time.Time `gorm:"not null"`
	EndedAt      *time.Time
	IsActive     bool  `gorm:"not null;default:true"`
	DurationMs   int64 `gorm:"default:0"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (CallSession) TableName() string {
	return "call_sessions"
}

type SignalingEventAudit struct {
	ID          uint      `gorm:"primaryKey"`
	EventType   string    `gorm:"index;not null"`
	UserID      string    `gorm:"index"`
	ClientID    string    `gorm:"index;not null"`
	RoomID      string    `gorm:"index"`
	Data        string    `gorm:"type:jsonb"` // Store raw JSON data
	Timestamp   int64     `gorm:"not null"`
	ProcessedAt time.Time `gorm:"not null"`
	CreatedAt   time.Time
}

func (SignalingEventAudit) TableName() string {
	return "signaling_events"
}
