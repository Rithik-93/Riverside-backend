package domain

import (
	"errors"
	"time"

	"github.com/lakeside/services/session-service/internal/infrastructure"
	"github.com/lakeside/services/session-service/pkg/types"
)

type AuthService interface {
	RegisterUser(email, username, fullName, password string) (*types.AuthResponse, error)
	LoginUser(email, password string) (*types.AuthResponse, error)
	RefreshToken(refreshToken string) (*types.AuthResponse, error)
	LogoutUser(refreshToken string) error
	ValidateToken(token string) (*types.TokenClaims, error)
	GetOAuthURL(provider string) (string, string, error)
	OAuthLogin(provider, code, state string) (*types.AuthResponse, error)
	ValidateSession(sessionID string) (bool, *infrastructure.SessionData, error)
	DeleteSession(sessionID string) error
}

func (a *authService) RegisterUser(email, username, fullName, password string) (*types.AuthResponse, error) {
	existingUser, _ := a.userRepo.GetByEmail(email)
	if existingUser != nil {
		return nil, errors.New("user with this email already exists")
	}

	existingUser, _ = a.userRepo.GetByUsername(username)
	if existingUser != nil {
		return nil, errors.New("username is already taken")
	}

	user, err := NewUser(email, username, fullName, password)
	if err != nil {
		return nil, err
	}

	err = a.userRepo.Create(user)
	if err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := a.tokenService.GenerateTokenPair(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, err
	}

	session := NewSession(user.ID, accessToken, refreshToken, 24*time.Hour)
	err = a.sessionRepo.Create(session)
	if err != nil {
		return nil, err
	}

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
		ExpiresIn:    24 * 60 * 60,
	}, nil
}


type authService struct {
	userRepo         UserRepository
	sessionRepo      SessionRepository
	tokenService     TokenService
	oauthService     OAuthService
	redisSessionSvc  *infrastructure.RedisSessionService
}

func NewAuthService(userRepo UserRepository, sessionRepo SessionRepository, tokenService TokenService, oauthService OAuthService, redisSessionSvc *infrastructure.RedisSessionService) AuthService {
	return &authService{
		userRepo:        userRepo,
		sessionRepo:     sessionRepo,
		tokenService:    tokenService,
		oauthService:    oauthService,
		redisSessionSvc: redisSessionSvc,
	}
}


func (a *authService) RegisterUserr(email, username, fullName, password string) (*types.AuthResponse, error) {
	return a.RegisterUser(email, username, fullName, password)
}

func (a *authService) LoginUser(email, password string) (*types.AuthResponse, error) {
	user, err := a.userRepo.GetByEmail(email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !user.CheckPassword(password) {
		return nil, errors.New("invalid credentials")
	}

	accessToken, refreshToken, err := a.tokenService.GenerateTokenPair(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, err
	}

	session := NewSession(user.ID, accessToken, refreshToken, 24*time.Hour)
	err = a.sessionRepo.Create(session)
	if err != nil {
		return nil, err
	}

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
		ExpiresIn:    24 * 60 * 60,
	}, nil
}

func (a *authService) RefreshToken(refreshToken string) (*types.AuthResponse, error) {
	_, err := a.tokenService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	session, err := a.sessionRepo.GetByRefreshToken(refreshToken)
	if err != nil || session == nil {
		return nil, errors.New("invalid refresh token")
	}

	if session.IsExpired() {
		return nil, errors.New("refresh token expired")
	}

	user, err := a.userRepo.GetByID(session.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	accessToken, newRefreshToken, err := a.tokenService.GenerateTokenPair(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, err
	}

	session.Refresh(accessToken, newRefreshToken, 24*time.Hour)
	err = a.sessionRepo.Update(session)
	if err != nil {
		return nil, err
	}

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
		ExpiresIn:    24 * 60 * 60,
	}, nil
}

func (a *authService) LogoutUser(refreshToken string) error {
	return a.sessionRepo.DeleteByRefreshToken(refreshToken)
}

func (a *authService) ValidateToken(token string) (*types.TokenClaims, error) {
	return a.tokenService.ValidateAccessToken(token)
}

func (a *authService) GetOAuthURL(provider string) (string, string, error) {
	return a.oauthService.GetAuthURL(provider)
}

func (a *authService) OAuthLogin(provider, code, state string) (*types.AuthResponse, error) {
	oauthUser, err := a.oauthService.ExchangeCode(provider, code, state)
	if err != nil {
		return nil, err
	}

	user, err := a.userRepo.GetByEmail(oauthUser.Email)
	if err != nil {
		user, err = NewUser(oauthUser.Email, oauthUser.Username, oauthUser.FullName, "")
		if err != nil {
			return nil, err
		}

		err = a.userRepo.Create(user)
		if err != nil {
			return nil, err
		}
	}

	accessToken, refreshToken, err := a.tokenService.GenerateTokenPair(user.ID, user.Email, user.Username)
	if err != nil {
		return nil, err
	}

	session := NewSession(user.ID, accessToken, refreshToken, 24*time.Hour)
	err = a.sessionRepo.Create(session)
	if err != nil {
		return nil, err
	}

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
		ExpiresIn:    24 * 60 * 60,
	}, nil
}

func (a *authService) ValidateSession(sessionID string) (bool, *infrastructure.SessionData, error) {
	if a.redisSessionSvc == nil {
		return false, nil, errors.New("Redis session service not available")
	}

	return a.redisSessionSvc.ValidateSession(sessionID)
}

func (a *authService) DeleteSession(sessionID string) error {
	if a.redisSessionSvc == nil {
		return errors.New("Redis session service not available")
	}

	return a.redisSessionSvc.DeleteSession(sessionID)
}
