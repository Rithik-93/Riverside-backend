package handlers

import (
	"net/http"

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

	// Basic validation
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Email and password are required", http.StatusBadRequest))
		return
	}

	// Call auth service
	authResponse, err := h.authService.LoginUser(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	// Return success response
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

	// Call auth service
	authResponse, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	// Return success response
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

	// Call auth service
	err := h.authService.LogoutUser(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Return success response
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

	// Call auth service
	authResponse, err := h.authService.OAuthLogin(req.Provider, req.Code, req.State)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Return success response
	c.JSON(http.StatusOK, types.SuccessResponse(authResponse, "OAuth login successful"))
}
