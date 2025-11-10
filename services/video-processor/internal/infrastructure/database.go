package infrastructure

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func ConnectDB() *gorm.DB {
	var dsn string
	
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		dsn = dbURL
	} else {
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"),
			os.Getenv("DB_PORT"),
		)
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Error,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormLogger,
		PrepareStmt:            true,
		SkipDefaultTransaction: false,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("✅ Database connected successfully")
	return db
}

