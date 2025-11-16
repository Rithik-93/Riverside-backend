package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lakeside/services/session-service/internal/domain"
	"github.com/lakeside/services/session-service/internal/infrastructure"
	"github.com/lakeside/services/session-service/internal/infrastructure/repository"
)

type PodcastHandler struct {
	podcastRepo   *repository.PodcastRepository
	recordingRepo *repository.RecordingRepository
	userRepo      *repository.UserRepository
	redisService  *infrastructure.RedisSessionService
}

func NewPodcastHandler(podcastRepo *repository.PodcastRepository, recordingRepo *repository.RecordingRepository, userRepo *repository.UserRepository, redisService *infrastructure.RedisSessionService) *PodcastHandler {
	return &PodcastHandler{
		podcastRepo:  podcastRepo,
		recordingRepo: recordingRepo,
		userRepo:     userRepo,
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

func (h *PodcastHandler) GetMyPodcasts(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	podcasts, err := h.podcastRepo.GetAllByHostUserID(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch podcasts"})
		return
	}

	type PodcastResponse struct {
		ID        int64     `json:"id"`
		CreatedAt string    `json:"created_at"`
		Recordings []gin.H `json:"recordings"`
	}

	var response []PodcastResponse
	for _, pod := range podcasts {
		recordings, _ := h.recordingRepo.GetByPodID(uint64(pod.ID))
		var recordingList []gin.H
		for _, rec := range recordings {
			links, _ := h.recordingRepo.GetUserLinksByRecordingID(rec.RecordingID)
			var userVideos []gin.H
			for _, link := range links {
				user, _ := h.userRepo.GetByID(link.UserID)
				userName := link.UserID
				if user != nil {
					userName = user.Username
				}
				userVideos = append(userVideos, gin.H{
					"user_id":      link.UserID,
					"user_name":    userName,
					"recording_id": link.RecordingID,
					"s3_url":       link.S3URL,
				})
			}
			recordingList = append(recordingList, gin.H{
				"recording_id": rec.RecordingID,
				"videos":       userVideos,
			})
		}
		response = append(response, PodcastResponse{
			ID:         pod.ID,
			CreatedAt: pod.CreatedAt.Format("2006-01-02 15:04:05"),
			Recordings: recordingList,
		})
	}

	c.JSON(http.StatusOK, gin.H{"podcasts": response})
}

func (h *PodcastHandler) GetPodcastRecordings(c *gin.Context) {
	podcastIDStr := c.Param("podcastId")
	podcastID, err := strconv.ParseInt(podcastIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid podcast ID"})
		return
	}

	recordings, err := h.recordingRepo.GetByPodID(uint64(podcastID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recordings"})
		return
	}

	var response []gin.H
	for _, rec := range recordings {
		links, _ := h.recordingRepo.GetUserLinksByRecordingID(rec.RecordingID)
		var userVideos []gin.H
		for _, link := range links {
			user, _ := h.userRepo.GetByID(link.UserID)
			userVideos = append(userVideos, gin.H{
				"user_id":      link.UserID,
				"user_name":    user.Username,
				"recording_id": link.RecordingID,
				"s3_url":       link.S3URL,
			})
		}
		response = append(response, gin.H{
			"recording_id": rec.RecordingID,
			"videos":       userVideos,
		})
	}

	c.JSON(http.StatusOK, gin.H{"recordings": response})
}

