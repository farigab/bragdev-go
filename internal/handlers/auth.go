package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/farigab/bragdoc/internal/config"
	"github.com/farigab/bragdoc/internal/domain"
	"github.com/farigab/bragdoc/internal/integration"
	"github.com/farigab/bragdoc/internal/repository"
	"github.com/farigab/bragdoc/internal/security"
)

// RegisterAuthRoutes registers authentication-related endpoints.
// oauth: integration.OAuthService (GitHub), jwtSvc: TokenService, userRepo/refreshRepo: persistence
func RegisterAuthRoutes(r chi.Router, cfg *config.Config, oauth integration.OAuthService, jwtSvc security.TokenService, userRepo repository.UserRepository, refreshRepo repository.RefreshTokenRepository) {
	r.Get("/api/auth/github", func(w http.ResponseWriter, req *http.Request) {
		state := uuid.New().String()
		redirectUri := cfg.GitHubRedirectURI
		if redirectUri == "" {
			redirectUri = "http://localhost:8080/api/auth/callback"
		}

		// Store state in a short-lived HttpOnly cookie for CSRF validation in the callback.
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			MaxAge:   300, // 5 minutes
			HttpOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})

		// Build authorize URL with proper URL-encoding for redirect_uri and state
		base := "https://github.com/login/oauth/authorize"
		q := url.Values{}
		q.Set("client_id", cfg.GitHubClientID)
		q.Set("redirect_uri", redirectUri)
		q.Set("scope", "read:user,user:email")
		q.Set("state", state)
		authorizeUrl := base + "?" + q.Encode()

		http.Redirect(w, req, authorizeUrl, http.StatusFound)
	})

	// Callback: exchange code, get profile, create user, generate tokens, set cookies and redirect
	r.Get("/api/auth/callback", func(w http.ResponseWriter, req *http.Request) {
		// Validate CSRF state before processing anything else.
		stateCookie, err := req.Cookie("oauth_state")
		if err != nil || stateCookie.Value == "" {
			http.Error(w, "missing state", http.StatusBadRequest)
			return
		}
		if stateParam := req.URL.Query().Get("state"); stateParam == "" || stateParam != stateCookie.Value {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		// Clear the state cookie immediately after validation.
		http.SetCookie(w, &http.Cookie{Name: "oauth_state", MaxAge: -1, Path: "/"})

		code := req.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "code missing", http.StatusBadRequest)
			return
		}

		redirectUri := cfg.GitHubRedirectURI
		if redirectUri == "" {
			redirectUri = "http://localhost:8080/api/auth/callback"
		}

		accessToken, err := oauth.ExchangeCodeForToken(code, redirectUri)
		if err != nil {
			log.Printf("error exchanging code: %v", err)
			http.Redirect(w, req, cfg.FrontendRedirectURI+"/login?error=auth_failed", http.StatusFound)
			return
		}

		profile, err := oauth.GetUserProfile(accessToken)
		if err != nil {
			log.Printf("error fetching profile: %v", err)
			http.Redirect(w, req, cfg.FrontendRedirectURI+"/login?error=auth_failed", http.StatusFound)
			return
		}

		loginRaw, _ := profile["login"].(string)
		nameRaw, _ := profile["name"].(string)
		avatarRaw, _ := profile["avatar_url"].(string)
		if nameRaw == "" {
			nameRaw = loginRaw
		}

		// create or update user
		user := domain.NewUser(loginRaw, nameRaw, avatarRaw)
		user.GitHubAccessToken = accessToken
		savedUser, err := userRepo.Save(user)
		if err != nil {
			log.Printf("error saving user: %v", err)
			http.Redirect(w, req, cfg.FrontendRedirectURI+"/login?error=auth_failed", http.StatusFound)
			return
		}

		// generate JWT
		jwtToken, err := jwtSvc.GenerateToken(savedUser.Login, map[string]interface{}{"name": savedUser.Name, "avatar": savedUser.AvatarURL})
		if err != nil {
			log.Printf("error generating jwt: %v", err)
			http.Redirect(w, req, cfg.FrontendRedirectURI+"/login?error=auth_failed", http.StatusFound)
			return
		}

		// create refresh token
		refreshTokenStr := uuid.New().String()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		rt := domain.NewRefreshToken(refreshTokenStr, savedUser.Login, expiresAt)
		_, err = refreshRepo.Save(rt)
		if err != nil {
			log.Printf("error saving refresh token: %v", err)
			http.Redirect(w, req, cfg.FrontendRedirectURI+"/login?error=auth_failed", http.StatusFound)
			return
		}

		// set cookies
		setCookie(w, "token", jwtToken, 15*60, cfg)
		setCookie(w, "refreshToken", refreshTokenStr, 7*24*60*60, cfg)

		log.Printf("User authenticated: %s", savedUser.Login)
		http.Redirect(w, req, cfg.FrontendRedirectURI, http.StatusFound)
	})

	// Refresh access token using refresh token cookie
	r.Post("/api/auth/refresh", func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("refreshToken")
		if err != nil || cookie.Value == "" {
			clearAuthCookies(w, cfg)
			http.Error(w, "no refresh token", http.StatusUnauthorized)
			return
		}

		oldRt, err := refreshRepo.FindByToken(cookie.Value)
		if err != nil {
			clearAuthCookies(w, cfg)
			http.Error(w, "invalid refresh token", http.StatusUnauthorized)
			return
		}
		if oldRt.Revoked || time.Now().After(oldRt.ExpiresAt) {
			clearAuthCookies(w, cfg)
			http.Error(w, "refresh token expired", http.StatusUnauthorized)
			return
		}

		user, err := userRepo.FindByLogin(oldRt.UserLogin)
		if err != nil {
			clearAuthCookies(w, cfg)
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}

		// generate new access token and new refresh token (rotate)
		jwtToken, err := jwtSvc.GenerateToken(user.Login, map[string]interface{}{"name": user.Name, "avatar": user.AvatarURL})
		if err != nil {
			http.Error(w, "failed to generate token", http.StatusInternalServerError)
			return
		}

		newToken := uuid.New().String()
		newExpires := time.Now().Add(7 * 24 * time.Hour)
		newRt := domain.NewRefreshToken(newToken, user.Login, newExpires)
		_, err = refreshRepo.Save(newRt)
		if err != nil {
			http.Error(w, "failed to save refresh token", http.StatusInternalServerError)
			return
		}

		// revoke/delete old token
		_ = refreshRepo.Delete(oldRt)

		setCookie(w, "token", jwtToken, 15*60, cfg)
		setCookie(w, "refreshToken", newToken, 7*24*60*60, cfg)

		w.WriteHeader(http.StatusOK)
	})

	// Logout - revoke refresh tokens and clear cookies
	r.Post("/api/auth/logout", func(w http.ResponseWriter, req *http.Request) {
		// try to extract user from token cookie
		tokenCookie, err := req.Cookie("token")
		if err == nil && tokenCookie.Value != "" {
			if login, err := jwtSvc.ExtractUserLogin(tokenCookie.Value); err == nil && login != "" {
				_ = refreshRepo.DeleteAllByUserLogin(login)
			}
		}
		clearAuthCookies(w, cfg)
		w.WriteHeader(http.StatusOK)
	})

	// Save GitHub token - expects JSON {"token":"..."}
	r.Post("/api/auth/github/token", func(w http.ResponseWriter, req *http.Request) {
		tokenCookie, err := req.Cookie("token")
		if err != nil || tokenCookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		login, err := jwtSvc.ExtractUserLogin(tokenCookie.Value)
		if err != nil || login == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var payload struct {
			Token string `json:"token"`
		}
		req.Body = http.MaxBytesReader(w, req.Body, maxBodyBytes)
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		u := &domain.User{Login: login, GitHubAccessToken: payload.Token}
		_, err = userRepo.Save(u)
		if err != nil {
			http.Error(w, "failed to save token", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Clear GitHub token
	r.Delete("/api/auth/github/token", func(w http.ResponseWriter, req *http.Request) {
		tokenCookie, err := req.Cookie("token")
		if err != nil || tokenCookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		login, err := jwtSvc.ExtractUserLogin(tokenCookie.Value)
		if err != nil || login == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		u := &domain.User{Login: login, GitHubAccessToken: ""}
		_, err = userRepo.Save(u)
		if err != nil {
			http.Error(w, "failed to clear token", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func setCookie(w http.ResponseWriter, name, value string, maxAge int, cfg *config.Config) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		Path:     "/",
		MaxAge:   maxAge,
		SameSite: parseSameSite(cfg.CookieSameSite),
	}

	// Only set Domain when explicitly configured and not localhost
	if cfg != nil && cfg.CookieDomain != "" && cfg.CookieDomain != "localhost" {
		cookie.Domain = cfg.CookieDomain
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
