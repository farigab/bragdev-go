// Package middleware provides HTTP middleware used across handlers.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/farigab/bragdev-go/internal/config"
	"github.com/farigab/bragdev-go/internal/domain"
	"github.com/farigab/bragdev-go/internal/repository"
	"github.com/farigab/bragdev-go/internal/security"
)

type contextKey string

// UserLoginKey is the context key for the authenticated user login.
const UserLoginKey contextKey = "userLogin"

// Auth is a chi middleware that validates the JWT cookie and stores the
// authenticated userLogin in the request context. Routes protected by this
// middleware can retrieve the login via UserLoginFromContext.
func Auth(jwtSvc security.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("token")
			if err != nil || cookie.Value == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userLogin, err := jwtSvc.ExtractUserLogin(cookie.Value)
			if err != nil || userLogin == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserLoginKey, userLogin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthWithRefresh is like Auth but attempts to rotate/refresh the JWT using
// the HttpOnly refreshToken cookie when the access token is missing or invalid.
// It sets new `token` and `refreshToken` cookies on success and continues the
// request with the authenticated user in context.
func AuthWithRefresh(cfg *config.Config, jwtSvc security.TokenService, userRepo repository.UserRepository, refreshRepo repository.RefreshTokenRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userLogin := extractValidLogin(jwtSvc, r); userLogin != "" {
				ctx := context.WithValue(r.Context(), UserLoginKey, userLogin)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			userLogin, err := rotateRefreshToken(cfg, jwtSvc, userRepo, refreshRepo, w, r)
			if err != nil {
				return // rotateRefreshToken already wrote the error response
			}
			ctx := context.WithValue(r.Context(), UserLoginKey, userLogin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractValidLogin reads the "token" cookie and returns the user login if valid,
// or an empty string when the token is absent or invalid.
func extractValidLogin(jwtSvc security.TokenService, r *http.Request) string {
	cookie, err := r.Cookie("token")
	if err != nil || cookie.Value == "" {
		return ""
	}
	userLogin, err := jwtSvc.ExtractUserLogin(cookie.Value)
	if err != nil {
		return ""
	}
	return userLogin
}

// rotateRefreshToken validates the refreshToken cookie, issues a new access token
// and refresh token pair, writes both cookies, and returns the authenticated user login.
// On any failure it writes the appropriate HTTP error and returns a non-nil error.
func rotateRefreshToken(cfg *config.Config, jwtSvc security.TokenService, userRepo repository.UserRepository, refreshRepo repository.RefreshTokenRepository, w http.ResponseWriter, r *http.Request) (string, error) {
	rtCookie, err := r.Cookie("refreshToken")
	if err != nil || rtCookie.Value == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", fmt.Errorf("missing refresh token cookie")
	}

	oldRt, err := refreshRepo.FindByToken(rtCookie.Value)
	if err != nil {
		clearAuthCookiesLocal(w, cfg)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", err
	}
	if oldRt.Revoked || time.Now().After(oldRt.ExpiresAt) {
		clearAuthCookiesLocal(w, cfg)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", fmt.Errorf("refresh token expired or revoked")
	}

	user, err := userRepo.FindByLogin(oldRt.UserLogin)
	if err != nil {
		clearAuthCookiesLocal(w, cfg)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", err
	}

	jwtToken, err := jwtSvc.GenerateToken(user.Login, map[string]interface{}{"name": user.Name, "avatar": user.AvatarURL})
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return "", err
	}

	newToken := uuid.New().String()
	newRt := domain.NewRefreshToken(newToken, user.Login, time.Now().Add(7*24*time.Hour))
	if _, err = refreshRepo.Save(newRt); err != nil {
		http.Error(w, "failed to save refresh token", http.StatusInternalServerError)
		return "", err
	}

	// Must succeed to prevent replay attacks — old token is only removed after the new one is saved.
	if err = refreshRepo.Delete(oldRt); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return "", err
	}

	setCookieLocal(w, "token", jwtToken, 15*60, cfg)
	setCookieLocal(w, "refreshToken", newToken, 7*24*60*60, cfg)
	return user.Login, nil
}

func setCookieLocal(w http.ResponseWriter, name, value string, maxAge int, cfg *config.Config) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		Path:     "/",
		MaxAge:   maxAge,
		SameSite: parseSameSiteLocal(cfg.CookieSameSite),
	}
	if cfg.CookieDomain != "" && cfg.CookieDomain != "localhost" {
		cookie.Domain = cfg.CookieDomain
	}
	http.SetCookie(w, cookie)
}

func clearAuthCookiesLocal(w http.ResponseWriter, cfg *config.Config) {
	setCookieLocal(w, "token", "", -1, cfg)
	setCookieLocal(w, "refreshToken", "", -1, cfg)
}

func parseSameSiteLocal(s string) http.SameSite {
	switch s {
	case "Strict":
		return http.SameSiteStrictMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// UserLoginFromContext extracts the authenticated user login from the request context.
// Returns the login and true if present, or empty string and false otherwise.
func UserLoginFromContext(ctx context.Context) (string, bool) {
	login, ok := ctx.Value(UserLoginKey).(string)
	return login, ok && login != ""
}
