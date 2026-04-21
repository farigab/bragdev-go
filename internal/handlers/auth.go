// Package handlers contains HTTP handler registrations and implementations.
package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/farigab/bragdev-go/internal/config"
	"github.com/farigab/bragdev-go/internal/domain"
	"github.com/farigab/bragdev-go/internal/integration"
	"github.com/farigab/bragdev-go/internal/logger"
	"github.com/farigab/bragdev-go/internal/repository"
	"github.com/farigab/bragdev-go/internal/security"
)

const (
	loginErrorURL = "/login?error=auth_failed"
	maxBodyBytes  = 1 << 20 // 1 MiB
)

// authHandler holds dependencies for all auth routes, avoiding re-capturing
// the same variables in every closure and making each handler independently testable.
type authHandler struct {
	cfg         *config.Config
	oauth       integration.OAuthService
	jwtSvc      security.TokenService
	userRepo    repository.UserRepository
	refreshRepo repository.RefreshTokenRepository
}

// RegisterAuthRoutes registers authentication-related endpoints.
func RegisterAuthRoutes(r chi.Router, cfg *config.Config, oauth integration.OAuthService, jwtSvc security.TokenService, userRepo repository.UserRepository, refreshRepo repository.RefreshTokenRepository) {
	h := &authHandler{cfg, oauth, jwtSvc, userRepo, refreshRepo}

	r.Get("/api/auth/github", h.handleGitHubLogin)
	r.Get("/api/auth/callback", h.handleGitHubCallback)
	r.Post("/api/auth/refresh", h.handleRefresh)
	r.Post("/api/auth/logout", h.handleLogout)
	r.Post("/api/auth/github/token", h.handleSaveGitHubToken)
	r.Delete("/api/auth/github/token", h.handleClearGitHubToken)
}

// handleGitHubLogin initiates the OAuth flow by redirecting to GitHub.
func (h *authHandler) handleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	state := uuid.New().String()

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   300,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	q := url.Values{}
	q.Set("client_id", h.cfg.GitHubClientID)
	q.Set("redirect_uri", h.resolveRedirectURI())
	q.Set("scope", "read:user,user:email")
	q.Set("state", state)

	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+q.Encode(), http.StatusFound)
}

// handleGitHubCallback completes the OAuth flow: validates state, exchanges code,
// fetches profile, upserts the user, issues JWT + refresh token, then redirects.
func (h *authHandler) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if err := h.validateOAuthState(w, r); err != nil {
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "code missing", http.StatusBadRequest)
		return
	}

	accessToken, err := h.oauth.ExchangeCodeForToken(code, h.resolveRedirectURI())
	if err != nil {
		logger.Errorw("error exchanging code", "err", err)
		h.redirectLoginError(w, r)
		return
	}

	savedUser, err := h.upsertUserFromOAuth(accessToken)
	if err != nil {
		h.redirectLoginError(w, r)
		return
	}

	if err := h.issueAuthCookies(w, savedUser); err != nil {
		h.redirectLoginError(w, r)
		return
	}

	logger.Infow("user authenticated", "login", savedUser.Login)
	http.Redirect(w, r, h.cfg.FrontendRedirectURI, http.StatusFound)
}

// handleRefresh rotates the refresh token and issues a new JWT.
func (h *authHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refreshToken")
	if err != nil || cookie.Value == "" {
		clearAuthCookies(w, h.cfg)
		http.Error(w, "no refresh token", http.StatusUnauthorized)
		return
	}

	oldRt, err := h.refreshRepo.FindByToken(cookie.Value)
	if err != nil {
		clearAuthCookies(w, h.cfg)
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	if oldRt.Revoked || time.Now().After(oldRt.ExpiresAt) {
		clearAuthCookies(w, h.cfg)
		http.Error(w, "refresh token expired", http.StatusUnauthorized)
		return
	}

	user, err := h.userRepo.FindByLogin(oldRt.UserLogin)
	if err != nil {
		clearAuthCookies(w, h.cfg)
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	if err := h.rotateTokens(w, user, oldRt); err != nil {
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleLogout revokes all refresh tokens for the user and clears auth cookies.
func (h *authHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if login := h.loginFromCookie(r); login != "" {
		_ = h.refreshRepo.DeleteAllByUserLogin(login)
	}
	clearAuthCookies(w, h.cfg)
	w.WriteHeader(http.StatusOK)
}

// handleSaveGitHubToken persists a manually-supplied GitHub access token for the current user.
func (h *authHandler) handleSaveGitHubToken(w http.ResponseWriter, r *http.Request) {
	login, ok := h.requireLogin(w, r)
	if !ok {
		return
	}

	var payload struct {
		Token string `json:"token"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if _, err := h.userRepo.Save(&domain.User{Login: login, GitHubAccessToken: payload.Token}); err != nil {
		http.Error(w, "failed to save token", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleClearGitHubToken removes the stored GitHub access token for the current user.
func (h *authHandler) handleClearGitHubToken(w http.ResponseWriter, r *http.Request) {
	login, ok := h.requireLogin(w, r)
	if !ok {
		return
	}

	if err := h.userRepo.ClearGitHubToken(login); err != nil {
		http.Error(w, "failed to clear token", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// --- private helpers ---------------------------------------------------------

// validateOAuthState checks the CSRF state cookie against the query parameter
// and immediately clears the cookie on success.
func (h *authHandler) validateOAuthState(w http.ResponseWriter, r *http.Request) error {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "missing state", http.StatusBadRequest)
		return err
	}

	stateParam := r.URL.Query().Get("state")
	if stateParam == "" || stateParam != stateCookie.Value {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return http.ErrNoCookie // non-nil sentinel; caller only checks nil/non-nil
	}

	http.SetCookie(w, &http.Cookie{Name: "oauth_state", MaxAge: -1, Path: "/"})
	return nil
}

// upsertUserFromOAuth fetches the GitHub profile for the given access token
// and saves (create or update) the corresponding User record.
func (h *authHandler) upsertUserFromOAuth(accessToken string) (*domain.User, error) {
	profile, err := h.oauth.GetUserProfile(accessToken)
	if err != nil {
		logger.Errorw("error fetching profile", "err", err)
		return nil, err
	}

	login, _ := profile["login"].(string)
	name, _ := profile["name"].(string)
	avatar, _ := profile["avatar_url"].(string)
	if name == "" {
		name = login
	}

	user := domain.NewUser(login, name, avatar)
	// Do not persist the GitHub access token when users authenticate via OAuth.
	// Users can explicitly store a token later via the dedicated endpoint.
	saved, err := h.userRepo.Save(user)
	if err != nil {
		logger.Errorw("error saving user", "err", err)
		return nil, err
	}
	return saved, nil
}

// issueAuthCookies generates a JWT and a fresh refresh token for the user,
// then writes both as HttpOnly cookies.
func (h *authHandler) issueAuthCookies(w http.ResponseWriter, user *domain.User) error {
	jwtToken, err := h.jwtSvc.GenerateToken(user.Login, map[string]interface{}{
		"name":   user.Name,
		"avatar": user.AvatarURL,
	})
	if err != nil {
		logger.Errorw("error generating jwt", "err", err)
		return err
	}

	refreshTokenStr, err := h.createRefreshToken(user.Login)
	if err != nil {
		logger.Errorw("error saving refresh token", "err", err)
		return err
	}

	setCookie(w, "token", jwtToken, 15*60, h.cfg)
	setCookie(w, "refreshToken", refreshTokenStr, 7*24*60*60, h.cfg)
	return nil
}

// rotateTokens issues a new JWT + refresh token and revokes the old refresh token.
func (h *authHandler) rotateTokens(w http.ResponseWriter, user *domain.User, oldRt *domain.RefreshToken) error {
	jwtToken, err := h.jwtSvc.GenerateToken(user.Login, map[string]interface{}{
		"name":   user.Name,
		"avatar": user.AvatarURL,
	})
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return err
	}

	newToken, err := h.createRefreshToken(user.Login)
	if err != nil {
		http.Error(w, "failed to save refresh token", http.StatusInternalServerError)
		return err
	}

	_ = h.refreshRepo.Delete(oldRt)

	setCookie(w, "token", jwtToken, 15*60, h.cfg)
	setCookie(w, "refreshToken", newToken, 7*24*60*60, h.cfg)
	return nil
}

// createRefreshToken persists a new 7-day refresh token and returns its string value.
func (h *authHandler) createRefreshToken(userLogin string) (string, error) {
	token := uuid.New().String()
	rt := domain.NewRefreshToken(token, userLogin, time.Now().Add(7*24*time.Hour))
	if _, err := h.refreshRepo.Save(rt); err != nil {
		return "", err
	}
	return token, nil
}

// requireLogin reads and validates the JWT cookie, writing 401 on failure.
// Returns the user's login and true on success.
func (h *authHandler) requireLogin(w http.ResponseWriter, r *http.Request) (string, bool) {
	cookie, err := r.Cookie("token")
	if err != nil || cookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	login, err := h.jwtSvc.ExtractUserLogin(cookie.Value)
	if err != nil || login == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return login, true
}

// loginFromCookie silently extracts the user login from the JWT cookie,
// returning an empty string on any error (used in non-critical paths like logout).
func (h *authHandler) loginFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("token")
	if err != nil || cookie.Value == "" {
		return ""
	}
	login, err := h.jwtSvc.ExtractUserLogin(cookie.Value)
	if err != nil {
		return ""
	}
	return login
}

// resolveRedirectURI returns the configured redirect URI or falls back to localhost.
func (h *authHandler) resolveRedirectURI() string {
	if h.cfg.GitHubRedirectURI != "" {
		return h.cfg.GitHubRedirectURI
	}
	return "http://localhost:8080/api/auth/callback"
}

// redirectLoginError sends the user to the frontend login error page.
func (h *authHandler) redirectLoginError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.cfg.FrontendRedirectURI+loginErrorURL, http.StatusFound)
}

// --- package-level cookie helpers --------------------------------------------

func setCookie(w http.ResponseWriter, name, value string, maxAge int, cfg *config.Config) {
	// safely read cfg fields only when cfg is not nil
	secure := false
	sameSite := http.SameSiteLaxMode
	var domain string
	if cfg != nil {
		secure = cfg.CookieSecure
		sameSite = parseSameSite(cfg.CookieSameSite)
		domain = cfg.CookieDomain
	}

	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
		MaxAge:   maxAge,
		SameSite: sameSite,
	}
	if domain != "" && domain != "localhost" {
		cookie.Domain = domain
	}
	http.SetCookie(w, cookie)
}

func clearAuthCookies(w http.ResponseWriter, cfg *config.Config) {
	setCookie(w, "token", "", 0, cfg)
	setCookie(w, "refreshToken", "", 0, cfg)
}

func parseSameSite(s string) http.SameSite {
	switch s {
	case "Strict":
		return http.SameSiteStrictMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
