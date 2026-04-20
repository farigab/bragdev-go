package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config armazena variáveis de ambiente usadas pela aplicação.
type Config struct {
	DBUrl               string
	JwtSecret           string
	GitHubClientID      string
	GitHubClientSecret  string
	GeminiApiKey        string
	GeminiApiUrl        string
	GeminiModel         string
	GitHubRedirectURI   string
	FrontendRedirectURI string
	CookieDomain        string
	CookieSecure        bool
	CookieSameSite      string
}

// Load carrega variáveis de ambiente (.env opcional) e retorna a configuração.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file loaded:", err)
	}
	gitHubRedirect := os.Getenv("GITHUB_OAUTH_REDIRECT_URI")
	if gitHubRedirect == "" {
		gitHubRedirect = "http://localhost:8080/api/auth/callback"
	}

	frontendRedirect := os.Getenv("OAUTH_FRONTEND_REDIRECT")
	if frontendRedirect == "" {
		frontendRedirect = "http://localhost:4200"
	}

	cookieDomain := os.Getenv("APP_COOKIE_DOMAIN")
	if cookieDomain == "" {
		cookieDomain = "localhost"
	}

	cookieSameSite := os.Getenv("APP_COOKIE_SAME_SITE")
	if cookieSameSite == "" {
		cookieSameSite = "Lax"
	}

	geminiUrl := os.Getenv("GEMINI_API_URL")
	if geminiUrl == "" {
		geminiUrl = "https://generativelanguage.googleapis.com/v1"
	}
	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-2.5-flash"
	}

	return &Config{
		DBUrl:               os.Getenv("DB_URL"),
		JwtSecret:           os.Getenv("JWT_SECRET"),
		GitHubClientID:      os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		GitHubClientSecret:  os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
		GeminiApiKey:        os.Getenv("GEMINI_API_KEY"),
		GeminiApiUrl:        geminiUrl,
		GeminiModel:         geminiModel,
		GitHubRedirectURI:   gitHubRedirect,
		FrontendRedirectURI: frontendRedirect,
		CookieDomain:        cookieDomain,
		CookieSecure:        os.Getenv("APP_COOKIE_SECURE") == "true",
		CookieSameSite:      cookieSameSite,
	}
}
