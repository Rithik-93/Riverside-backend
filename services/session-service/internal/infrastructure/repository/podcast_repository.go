package repository

import (
	"github.com/lakeside/services/session-service/internal/domain"
	"gorm.io/gorm"
)

type PodcastRepository struct {
	db *gorm.DB
}

func NewPodcastRepository(db *gorm.DB) *PodcastRepository {
	return &PodcastRepository{db: db}
}

func (r *PodcastRepository) Create(podcast *domain.Podcast) error {
	return r.db.Create(podcast).Error
}

func (r *PodcastRepository) GetByHostUserID(hostUserID string) (*domain.Podcast, error) {
	var podcast domain.Podcast
	err := r.db.Where("host_user_id = ? AND is_active = ?", hostUserID, true).
		Order("created_at DESC").
		First(&podcast).Error
	
	if err != nil {
		return nil, err
	}
	
	return &podcast, nil
}

func (r *PodcastRepository) GetByID(id int64) (*domain.Podcast, error) {
	var podcast domain.Podcast
	err := r.db.Where("id = ? AND is_active = ?", id, true).First(&podcast).Error
	
	if err != nil {
		return nil, err
	}
	
	return &podcast, nil
}

func (r *PodcastRepository) ExistsByID(id int64) (bool, error) {
	var count int64
	err := r.db.Model(&domain.Podcast{}).
		Where("id = ? AND is_active = ?", id, true).
		Count(&count).Error
	
	if err != nil {
		return false, err
	}
	
	return count > 0, nil
}

func (r *PodcastRepository) GetAllByHostUserID(hostUserID string) ([]domain.Podcast, error) {
	var podcasts []domain.Podcast
	err := r.db.Where("host_user_id = ? AND is_active = ?", hostUserID, true).
		Order("created_at DESC").
		Find(&podcasts).Error
	
	return podcasts, err
}

