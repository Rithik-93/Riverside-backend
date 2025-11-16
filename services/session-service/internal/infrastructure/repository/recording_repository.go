package repository

import (
	"github.com/lakeside/services/session-service/internal/infrastructure"
	"gorm.io/gorm"
)

type RecordingRepository struct {
	db *gorm.DB
}

func NewRecordingRepository(db *gorm.DB) *RecordingRepository {
	return &RecordingRepository{db: db}
}

func (r *RecordingRepository) GetByPodID(podID uint64) ([]infrastructure.Recording, error) {
	var recordings []infrastructure.Recording
	err := r.db.Where("pod_id = ? AND state = ?", podID, "completed").
		Order("created_at DESC").
		Find(&recordings).Error
	return recordings, err
}

func (r *RecordingRepository) GetUserLinksByRecordingID(recordingID string) ([]infrastructure.UserRecordingLink, error) {
	var links []infrastructure.UserRecordingLink
	err := r.db.Where("recording_id = ?", recordingID).Find(&links).Error
	return links, err
}



