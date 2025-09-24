package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lakeside/services/session-service/internal/domain"
	"github.com/lakeside/services/session-service/pkg/types"
)

type AuthHandler struct {
	authService domain.AuthService
}

func NewAuthHandler(authService domain.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req types.UserRegistration
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if req.Email == "" || req.Username == "" || req.FullName == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("All fields are required", http.StatusBadRequest))
		return
	}

	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Password must be at least 8 characters", http.StatusBadRequest))
		return
	}

	authResponse, err := h.authService.RegisterUser(req.Email, req.Username, req.FullName, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	c.JSON(http.StatusCreated, types.SuccessResponse(authResponse, "User registered successfully"))
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req types.UserLogin
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Email and password are required", http.StatusBadRequest))
		return
	}

	authResponse, err := h.authService.LoginUser(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(authResponse, "Login successful"))
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req types.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if req.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Refresh token is required", http.StatusBadRequest))
		return
	}

	authResponse, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(authResponse, "Token refreshed successfully"))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req types.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if req.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Refresh token is required", http.StatusBadRequest))
		return
	}

	err := h.authService.LogoutUser(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(nil, "Logout successful"))
}

func (h *AuthHandler) OAuthLogin(c *gin.Context) {
	var req types.OAuthSession
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if req.Provider == "" || req.Code == "" || req.State == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Provider, code, and state are required", http.StatusBadRequest))
		return
	}


	authResponse, err := h.authService.OAuthLogin(req.Provider, req.Code, req.State)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}


	c.JSON(http.StatusOK, types.SuccessResponse(authResponse, "OAuth login successful"))
}

func (h *AuthHandler) ValidateToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse("Authorization header required", http.StatusUnauthorized))
		return
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse("Invalid authorization header format", http.StatusUnauthorized))
		return
	}

	token := tokenParts[1]
	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse("Invalid token", http.StatusUnauthorized))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(claims, "Token is valid"))
}

func (h *AuthHandler) CreateSession(c *gin.Context) {
	// Get user ID from JWT token
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse("User not authenticated", http.StatusUnauthorized))
		return
	}

	var req struct {
		RoomID string `json:"room_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Room ID is required", http.StatusBadRequest))
		return
	}

	authHeader := c.GetHeader("Authorization")
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse("Invalid authorization header", http.StatusUnauthorized))
		return
	}

	sessionID, err := h.authService.CreateSession(userID.(string), tokenParts[1], req.RoomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	c.JSON(http.StatusCreated, types.SuccessResponse(gin.H{"session_id": sessionID}, "Session created successfully"))
}

func (h *AuthHandler) ValidateSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Session ID is required", http.StatusBadRequest))
		return
	}

	valid, sessionData, err := h.authService.ValidateSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, types.NewErrorResponse("Session not found or invalid", http.StatusNotFound))
		return
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse("Session is inactive or expired", http.StatusUnauthorized))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(sessionData, "Session is valid"))
}

func (h *AuthHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Session ID is required", http.StatusBadRequest))
		return
	}

	err := h.authService.DeleteSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(nil, "Session deleted successfully"))
}