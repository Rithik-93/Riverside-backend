package domain

import (
	"errors"
	"time"

	"github.com/lakeside/services/session-service/pkg/types"
)

type AuthService interface {
	RegisterUser(email, username, fullName, password string) (*types.AuthResponse, error)
	LoginUser(email, password string) (*types.AuthResponse, error)
	RefreshToken(refreshToken string) (*types.AuthResponse, error)
	LogoutUser(refreshToken string) error
	ValidateToken(token string) (*types.TokenClaims, error)
	OAuthLogin(provider, code, state string) (*types.AuthResponse, error)
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
	userRepo    UserRepository
	sessionRepo SessionRepository
	tokenService TokenService
	oauthService OAuthService
}

func NewAuthService(userRepo UserRepository, sessionRepo SessionRepository, tokenService TokenService, oauthService OAuthService) AuthService {
	return &authService{
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		tokenService: tokenService,
		oauthService: oauthService,
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
	claims, err := a.tokenService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	userID := claims.UserID
	if userID == "" {
		return nil, errors.New("invalid user ID")
	}

	user, err := a.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	session, err := a.sessionRepo.GetByRefreshToken(refreshToken)
	if err != nil || session == nil {
		return nil, errors.New("invalid refresh token")
	}

	if session.IsExpired() {
		return nil, errors.New("refresh token expired")
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
