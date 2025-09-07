package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/lakeside/services/session-service/pkg/types"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type OAuthService struct {
	googleConfig *oauth2.Config
	stateStore   map[string]bool // In production, use Redis or database
}

func NewOAuthService(clientID, clientSecret, redirectURL string) *OAuthService {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: google.Endpoint,
	}

	return &OAuthService{
		googleConfig: config,
		stateStore:   make(map[string]bool),
	}
}

func (o *OAuthService) GetAuthURL(provider string) (string, string, error) {
	if provider != "google" {
		return "", "", fmt.Errorf("unsupported provider: %s", provider)
	}

	state := o.generateRandomState()
	o.stateStore[state] = true

	authURL := o.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return authURL, state, nil
}

func (o *OAuthService) ExchangeCode(provider, code, state string) (*types.OAuthUser, error) {
	if provider != "google" {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	if !o.ValidateState(state) {
		return nil, fmt.Errorf("invalid state parameter")
	}

	token, err := o.googleConfig.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %v", err)
	}

	userInfo, err := o.getGoogleUserInfo(token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}

	delete(o.stateStore, state)

	return userInfo, nil
}

func (o *OAuthService) ValidateState(state string) bool {
	valid, exists := o.stateStore[state]
	return exists && valid
}

func (o *OAuthService) generateRandomState() string {
	stateBytes := make([]byte, 32)
	rand.Read(stateBytes)
	return base64.URLEncoding.EncodeToString(stateBytes)
}

func (o *OAuthService) getGoogleUserInfo(accessToken string) (*types.OAuthUser, error) {
	url := "https://www.googleapis.com/oauth2/v2/userinfo"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: %s", resp.Status)
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, err
	}

	username := strings.Split(googleUser.Email, "@")[0]

	return &types.OAuthUser{
		Email:      googleUser.Email,
		Username:   username,
		FullName:   googleUser.Name,
		Picture:    googleUser.Picture,
		Provider:   "google",
		ProviderID: googleUser.ID,
	}, nil
}
