package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

// OAuthService defines required operations to exchange code and fetch profile.
type OAuthService interface {
	ExchangeCodeForToken(code string, redirectURI string) (string, error)
	GetUserProfile(accessToken string) (map[string]interface{}, error)
}

// GitHubOAuthService is a minimal implementation that uses HTTP to call GitHub.
// Note: production code should handle retries, errors and use a typed struct for profile.
type GitHubOAuthService struct {
	client       *http.Client
	clientID     string
	clientSecret string
}

func NewGitHubOAuthService(clientID, clientSecret string) *GitHubOAuthService {
	return &GitHubOAuthService{client: http.DefaultClient, clientID: clientID, clientSecret: clientSecret}
}

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
	defer resp.Body.Close()

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

	// fallback parse as form-encoded
	vals, err := url.ParseQuery(string(respBody))
	if err == nil {
		if at := vals.Get("access_token"); at != "" {
			return at, nil
		}
	}

	return "", errors.New("access_token not found in response")
}

func (s *GitHubOAuthService) GetUserProfile(accessToken string) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, errors.New("non-2xx from github user api")
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
