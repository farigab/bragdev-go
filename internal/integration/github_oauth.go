package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

// OAuthService defines required operations to exchange code and fetch profile.
type OAuthService interface {
	ExchangeCodeForToken(code string, redirectURI string) (string, error)
	GetUserProfile(accessToken string) (map[string]interface{}, error)
}

// GitHubOAuthService is a minimal implementation that calls the GitHub OAuth API.
type GitHubOAuthService struct {
	client       *http.Client
	clientID     string
	clientSecret string
}

// NewGitHubOAuthService creates a new GitHubOAuthService.
// Uses an explicit 15-second timeout — http.DefaultClient has no timeout
// and would block goroutines indefinitely if GitHub is unresponsive.
func NewGitHubOAuthService(clientID, clientSecret string) *GitHubOAuthService {
	return &GitHubOAuthService{
		client:       &http.Client{Timeout: 15 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// ExchangeCodeForToken exchanges an OAuth code for an access token.
func (s *GitHubOAuthService) ExchangeCodeForToken(code string, redirectURI string) (string, error) {
	payload := map[string]string{
		"client_id":     s.clientID,
		"client_secret": s.clientSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer closeBody(resp.Body)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode/100 != 2 {
		return "", errors.New("non-2xx from github token endpoint: " + string(respBody))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		if at, ok := parsed["access_token"].(string); ok && at != "" {
			return at, nil
		}
	}

	// Fallback: GitHub may return form-encoded on some error paths.
	vals, err := url.ParseQuery(string(respBody))
	if err == nil {
		if at := vals.Get("access_token"); at != "" {
			return at, nil
		}
	}

	return "", errors.New("access_token not found in response")
}

// GetUserProfile retrieves the GitHub profile for the given access token.
func (s *GitHubOAuthService) GetUserProfile(accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeBody(resp.Body)

	if resp.StatusCode/100 != 2 {
		return nil, errors.New("non-2xx from github user api")
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
