// Package config handles loading application configuration from environment.
package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config stores environment variables used by the application.
type Config struct {
	SQLiteCloudURL      string
	JwtSecret           string
	GitHubClientID      string
	GitHubClientSecret  string
	GeminiAPIKey        string
	GeminiAPIURL        string
	GeminiModel         string
	GitHubRedirectURI   string
	FrontendRedirectURI string
	CookieDomain        string
	CookieSecure        bool
	CookieSameSite      string
	LogLevel            string
	Migrate             bool // run SQL migrations on startup when true
}

// Load reads environment variables (.env optional) and returns the config.
func Load() *Config {
	_ = godotenv.Load()

	gitHubRedirect := defaultString(os.Getenv("GITHUB_OAUTH_REDIRECT_URI"), "http://localhost:8080/api/auth/callback")
	frontendRedirect := defaultString(os.Getenv("OAUTH_FRONTEND_REDIRECT"), "http://localhost:4200")
	cookieDomain := defaultString(os.Getenv("APP_COOKIE_DOMAIN"), "localhost")
	cookieSameSite := defaultString(os.Getenv("APP_COOKIE_SAME_SITE"), "Lax")
	geminiURL := defaultString(os.Getenv("GEMINI_API_URL"), "https://generativelanguage.googleapis.com/v1")
	geminiModel := defaultString(os.Getenv("GEMINI_MODEL"), "gemini-2.5-flash")

	return &Config{
		SQLiteCloudURL:      os.Getenv("DB_URL"),
		JwtSecret:           os.Getenv("JWT_SECRET"),
		GitHubClientID:      os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		GitHubClientSecret:  os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
		GeminiAPIKey:        os.Getenv("GEMINI_API_KEY"),
		GeminiAPIURL:        geminiURL,
		GeminiModel:         geminiModel,
		GitHubRedirectURI:   gitHubRedirect,
		FrontendRedirectURI: frontendRedirect,
		CookieDomain:        cookieDomain,
		CookieSecure:        os.Getenv("APP_COOKIE_SECURE") == "true",
		CookieSameSite:      cookieSameSite,
		LogLevel:            defaultString(os.Getenv("LOG_LEVEL"), "info"),
		Migrate:             strings.ToLower(os.Getenv("MIGRATE")) == "true",
	}
}

func defaultString(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
