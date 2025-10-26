package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lakeside/services/session-service/internal/domain"
	"github.com/lakeside/services/session-service/internal/infrastructure"
	"github.com/lakeside/services/session-service/internal/infrastructure/repository"
)

type PodcastHandler struct {
	podcastRepo  *repository.PodcastRepository
	redisService *infrastructure.RedisSessionService
}

func NewPodcastHandler(podcastRepo *repository.PodcastRepository, redisService *infrastructure.RedisSessionService) *PodcastHandler {
	return &PodcastHandler{
		podcastRepo:  podcastRepo,
		redisService: redisService,
	}
}

func generatePodcastID() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (h *PodcastHandler) CreatePodcast(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	podcast := domain.NewPodcast(userID.(string))

	err := h.podcastRepo.Create(podcast)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create podcast"})
		return
	}

	podcastID := generatePodcastID()
	
	//setting the podcast to redis after pod creation for future pod validation
	err = h.redisService.SetPodcastHost(podcastID, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register podcast"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"podcast_id":   podcastID,
		"host_user_id": userID.(string),
	})
}

func (h *PodcastHandler) CheckPodcast(c *gin.Context) {
	podcastID := c.Param("podcastId")
	if podcastID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Podcast ID required"})
		return
	}

	hostUserID, err := h.redisService.GetPodcastHost(podcastID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check podcast"})
		return
	}

	if hostUserID == "" {
		c.JSON(http.StatusNotFound, gin.H{"exists": false, "error": "Studio not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"exists": true, "host_user_id": hostUserID})
}

