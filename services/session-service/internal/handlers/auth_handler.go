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

	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Email and password are required", http.StatusBadRequest))
		return
	}

	authResponse, err := h.authService.LoginUser(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	c.SetCookie("access_token", authResponse.AccessToken, 24*60*60, "/", "", false, true)  // 24 hours, HTTP-only
	c.SetCookie("refresh_token", authResponse.RefreshToken, 7*24*60*60, "/", "", false, true)  // 7 days, HTTP-only

	c.JSON(http.StatusOK, types.SuccessResponse(authResponse, "Login successful"))
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Refresh token is required", http.StatusBadRequest))
		return
	}

	authResponse, err := h.authService.RefreshToken(refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse(err.Error(), http.StatusUnauthorized))
		return
	}

	c.SetCookie("access_token", authResponse.AccessToken, 24*60*60, "/", "", false, true)  // 24 hours, HTTP-only
	c.SetCookie("refresh_token", authResponse.RefreshToken, 7*24*60*60, "/", "", false, true)  // 7 days, HTTP-only

	c.JSON(http.StatusOK, types.SuccessResponse(authResponse, "Token refreshed successfully"))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Refresh token is required", http.StatusBadRequest))
		return
	}

	err = h.authService.LogoutUser(refreshToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)

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
	token, err := c.Cookie("access_token")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse("Access token required", http.StatusUnauthorized))
		return
	}

	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.NewErrorResponse("Invalid token", http.StatusUnauthorized))
		return
	}

	c.JSON(http.StatusOK, types.SuccessResponse(claims, "Token is valid"))
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

func (h *AuthHandler) GetOAuthURL(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse("Provider is required", http.StatusBadRequest))
		return
	}

	authURL, state, err := h.authService.GetOAuthURL(provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	response := types.OAuthURLResponse{
		AuthURL: authURL,
		State:   state,
	}

	c.JSON(http.StatusOK, types.SuccessResponse(response, "OAuth URL generated successfully"))
}