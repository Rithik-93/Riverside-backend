package auth

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lakeside/services/session-service/internal/domain"
	"github.com/lakeside/services/session-service/internal/infrastructure/database"
	"github.com/lakeside/services/session-service/internal/infrastructure/repository"
	"github.com/lakeside/services/session-service/internal/service"
	"github.com/markbates/goth/gothic"
)

func BeginAuth(c *gin.Context) {
	provider := c.Param("provider")
	reqWithProvider := c.Request.WithContext(context.WithValue(c.Request.Context(), gothic.ProviderParamKey, provider))
	gothic.BeginAuthHandler(c.Writer, reqWithProvider)
}

func AuthController(c *gin.Context) {
	provider := c.Param("provider")
	reqWithProvider := c.Request.WithContext(context.WithValue(c.Request.Context(), gothic.ProviderParamKey, provider))
	gothUser, err := gothic.CompleteUserAuth(c.Writer, reqWithProvider)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	db := database.Connect()
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	accessSecret := os.Getenv("JWT_ACCESS_SECRET")
	if accessSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT_ACCESS_SECRET not configured"})
		return
	}

	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	if refreshSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT_REFRESH_SECRET not configured"})
		return
	}

	tokenService := service.NewTokenService(accessSecret, refreshSecret)

	// Generate username from email, but make it unique if needed
	baseUsername := strings.Split(gothUser.Email, "@")[0]
	username := baseUsername

	existingUser, err := userRepo.GetByEmail(gothUser.Email)
	if err != nil {
		_, err := userRepo.GetByUsername(username)
		if err == nil {
			username = baseUsername + "_" + strings.ToLower(gothUser.UserID[:8])
		}

		fullName := gothUser.Name
		if fullName == "" {
			fullName = username
		}

		newUser, err := domain.NewUser(gothUser.Email, username, fullName, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
			return
		}

		err = userRepo.Create(newUser)
		if err != nil {
			if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
				username = baseUsername + "_" + strings.ToLower(gothUser.UserID)
				newUser.Username = username
				err = userRepo.Create(newUser)
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user: " + err.Error()})
				return
			}
		}
		existingUser = newUser
	} else {
		// User exists - only update full name if provided, don't change username
		if gothUser.Name != "" && gothUser.Name != existingUser.FullName {
			existingUser.FullName = gothUser.Name
			err = userRepo.Update(existingUser)
			if err != nil {
				log.Printf("Warning: Failed to update user profile: %v", err)
			}
		}
	}

	accessToken, refreshToken, err := tokenService.GenerateTokenPair(existingUser.ID, existingUser.Email, existingUser.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	session := domain.NewSession(existingUser.ID, accessToken, refreshToken, 24*time.Hour)
	err = sessionRepo.Create(session)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	cookieDomain := os.Getenv("COOKIE_DOMAIN")
	secure := os.Getenv("COOKIE_SECURE") == "true"

	c.SetCookie("access_token", "", -1, "/", "", secure, true)
	c.SetCookie("refresh_token", "", -1, "/", "", secure, true)

	if cookieDomain != "" {
		c.SetCookie("access_token", "", -1, "/", cookieDomain, secure, true)
		c.SetCookie("refresh_token", "", -1, "/", cookieDomain, secure, true)
	}

	c.SetCookie("access_token", accessToken, 24*60*60, "/", cookieDomain, secure, true)
	c.SetCookie("refresh_token", refreshToken, 7*24*60*60, "/", cookieDomain, secure, true)

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173/dashboard/home"
	}
	c.Redirect(http.StatusFound, frontendURL)
}
