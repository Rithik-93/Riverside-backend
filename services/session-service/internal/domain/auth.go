package domain

import (
	"errors"
	"time"

	"github.com/lakeside/services/session-service/pkg/types"
)

// AuthService represents the authentication service interface
type AuthService interface {
	RegisterUser(email, username, fullName, password string) (*types.AuthResponse, error)
	LoginUser(email, password string) (*types.AuthResponse, error)
	RefreshToken(refreshToken string) (*types.AuthResponse, error)
	LogoutUser(refreshToken string) error
	ValidateToken(token string) (*types.TokenClaims, error)
	OAuthLogin(provider, code, state string) (*types.AuthResponse, error)
}

// authService implements the AuthService interface
type authService struct {
	userRepo    UserRepository
	sessionRepo SessionRepository
	tokenService TokenService
	oauthService OAuthService
}

// NewAuthService creates a new authentication service
func NewAuthService(userRepo UserRepository, sessionRepo SessionRepository, tokenService TokenService, oauthService OAuthService) AuthService {
	return &authService{
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		tokenService: tokenService,
		oauthService: oauthService,
	}
}

// RegisterUser handles user registration
func (a *authService) RegisterUser(email, username, fullName, password string) (*types.AuthResponse, error) {
	// Check if user already exists
	existingUser, _ := a.userRepo.GetByEmail(email)
	if existingUser != nil {
		return nil, errors.New("user with this email already exists")
	}

	existingUser, _ = a.userRepo.GetByUsername(username)
	if existingUser != nil {
		return nil, errors.New("username is already taken")
	}

	// Create new user
	user, err := NewUser(email, username, fullName, password)
	if err != nil {
		return nil, err
	}

	// Save user to repository
	err = a.userRepo.Create(user)
	if err != nil {
		return nil, err
	}

	// Generate tokens
	accessToken, refreshToken, err := a.tokenService.GenerateTokenPair(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, err
	}

	// Create session
	session := NewSession(user.ID, accessToken, refreshToken, 24*time.Hour)
	err = a.sessionRepo.Create(session)
	if err != nil {
		return nil, err
	}

	// Create user response
	userResponse := types.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return &types.AuthResponse{
		User:         userResponse,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    24 * 60 * 60, // 24 hours in seconds
	}, nil
}

func (a *authService) RegisterUserr(email, username, fullName, password string) (*types.AuthResponse, error) {
	return a.RegisterUser(email, username, fullName, password)
}

// LoginUser handles user login
func (a *authService) LoginUser(email, password string) (*types.AuthResponse, error) {
	// Get user by email
	user, err := a.userRepo.GetByEmail(email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Check password
	if !user.CheckPassword(password) {
		return nil, errors.New("invalid credentials")
	}

	// Generate tokens
	accessToken, refreshToken, err := a.tokenService.GenerateTokenPair(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, err
	}

	// Create session
	session := NewSession(user.ID, accessToken, refreshToken, 24*time.Hour)
	err = a.sessionRepo.Create(session)
	if err != nil {
		return nil, err
	}

	// Create user response
	userResponse := types.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return &types.AuthResponse{
		User:         userResponse,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    24 * 60 * 60, // 24 hours in seconds
	}, nil
}

// RefreshToken handles token refresh
func (a *authService) RefreshToken(refreshToken string) (*types.AuthResponse, error) {
	// Validate refresh token
	claims, err := a.tokenService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Get user
	userID := claims.UserID
	if userID == "" {
		return nil, errors.New("invalid user ID")
	}

	user, err := a.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Get session
	session, err := a.sessionRepo.GetByRefreshToken(refreshToken)
	if err != nil || session == nil {
		return nil, errors.New("invalid refresh token")
	}

	// Check if session is expired
	if session.IsExpired() {
		return nil, errors.New("refresh token expired")
	}

	// Generate new tokens
	accessToken, newRefreshToken, err := a.tokenService.GenerateTokenPair(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, err
	}

	// Update session
	session.Refresh(accessToken, newRefreshToken, 24*time.Hour)
	err = a.sessionRepo.Update(session)
	if err != nil {
		return nil, err
	}

	// Create user response
	userResponse := types.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return &types.AuthResponse{
		User:         userResponse,
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    24 * 60 * 60, // 24 hours in seconds
	}, nil
}

// LogoutUser handles user logout
func (a *authService) LogoutUser(refreshToken string) error {
	// Delete session
	return a.sessionRepo.DeleteByRefreshToken(refreshToken)
}

// ValidateToken validates an access token
func (a *authService) ValidateToken(token string) (*types.TokenClaims, error) {
	return a.tokenService.ValidateAccessToken(token)
}

// OAuthLogin handles OAuth authentication
func (a *authService) OAuthLogin(provider, code, state string) (*types.AuthResponse, error) {
	// Exchange code for tokens
	oauthUser, err := a.oauthService.ExchangeCode(provider, code, state)
	if err != nil {
		return nil, err
	}

	// Check if user exists
	user, err := a.userRepo.GetByEmail(oauthUser.Email)
	if err != nil {
		// Create new user if doesn't exist
		user, err = NewUser(oauthUser.Email, oauthUser.Username, oauthUser.FullName, "")
		if err != nil {
			return nil, err
		}

		err = a.userRepo.Create(user)
		if err != nil {
			return nil, err
		}
	}

	// Generate tokens
	accessToken, refreshToken, err := a.tokenService.GenerateTokenPair(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, err
	}

	// Create session
	session := NewSession(user.ID, accessToken, refreshToken, 24*time.Hour)
	err = a.sessionRepo.Create(session)
	if err != nil {
		return nil, err
	}

	// Create user response
	userResponse := types.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	return &types.AuthResponse{
		User:         userResponse,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    24 * 60 * 60, // 24 hours in seconds
	}, nil
}
