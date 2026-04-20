package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/farigab/bragdoc/internal/config"
	"github.com/farigab/bragdoc/internal/handlers"
	"github.com/farigab/bragdoc/internal/integration"
	appMiddleware "github.com/farigab/bragdoc/internal/middleware"
	"github.com/farigab/bragdoc/internal/repository"
	"github.com/farigab/bragdoc/internal/security"
	"github.com/farigab/bragdoc/internal/usecase"
)

func main() {
	cfg := config.Load()

	db, err := sqlx.Connect("postgres", cfg.DBUrl)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	// Optionally run migrations if MIGRATE=true
	if strings.ToLower(os.Getenv("MIGRATE")) == "true" {
		if err := runMigrations(db, "db/migrations"); err != nil {
			log.Fatalf("migrations failed: %v", err)
		}
	}

	r := chi.NewRouter()

	// CORS middleware
	r.Use(corsMiddleware(cfg))

	// health
	r.Get("/api/health", handlers.HealthHandler)

	// wiring
	userRepo := repository.NewPostgresUserRepo(db)
	achievementRepo := repository.NewPostgresAchievementRepo(db)
	refreshRepo := repository.NewPostgresRefreshTokenRepo(db)

	jwtSvc := security.NewJWTService(cfg.JwtSecret, 900) // 15 minutes
	oauthSvc := integration.NewGitHubOAuthService(cfg.GitHubClientID, cfg.GitHubClientSecret)
	geminiClient := integration.NewGeminiClient(cfg.GeminiApiKey, cfg.GeminiApiUrl, cfg.GeminiModel)
	fetcherFactory := integration.GitHubClientFactory{}
	reportSvc := usecase.NewReportService(userRepo, achievementRepo, fetcherFactory, geminiClient)

	// Public auth routes (OAuth flow + token refresh)
	handlers.RegisterAuthRoutes(r, cfg, oauthSvc, jwtSvc, userRepo, refreshRepo)

	// Protected routes — require a valid JWT cookie
	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthWithRefresh(cfg, jwtSvc, userRepo, refreshRepo))
		handlers.RegisterAchievementRoutes(r, achievementRepo)
		handlers.RegisterUserRoutes(r, userRepo)
		handlers.RegisterGitHubRoutes(r, userRepo)
		handlers.RegisterReportRoutes(r, reportSvc)
	})

	// Background job: cleanup expired refresh tokens every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := refreshRepo.DeleteExpiredTokens(); err != nil {
				log.Printf("failed cleaning expired refresh tokens: %v", err)
			}
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on :%s", port)
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to bind port: %v", err)
	}

	log.Printf("Server started on :%s", port)
	if err := http.Serve(ln, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func runMigrations(db *sqlx.DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		p := dir + "/" + n
		b, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", n, err)
		}
		sql := string(b)
		if strings.TrimSpace(sql) == "" {
			continue
		}
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("exec migration %s: %w", n, err)
		}
		log.Printf("applied migration: %s", n)
	}
	return nil
}

// corsMiddleware returns a chi middleware that sets CORS headers and
// handles preflight OPTIONS requests. It uses FrontendRedirectURI
// from config if present, otherwise allows all origins.
func corsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	// Build allowed origins list from config.FrontendRedirectURI (comma-separated)
	// or from env APP_ALLOWED_ORIGINS as a fallback.
	var allowedOrigins []string
	if cfg != nil && cfg.FrontendRedirectURI != "" {
		for _, p := range strings.Split(cfg.FrontendRedirectURI, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if u, err := url.Parse(p); err == nil && u.Scheme != "" && u.Host != "" {
				allowedOrigins = append(allowedOrigins, u.Scheme+"://"+u.Host)
			} else {
				allowedOrigins = append(allowedOrigins, p)
			}
		}
	}
	if len(allowedOrigins) == 0 {
		if env := os.Getenv("APP_ALLOWED_ORIGINS"); env != "" {
			for _, p := range strings.Split(env, ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if u, err := url.Parse(p); err == nil && u.Scheme != "" && u.Host != "" {
					allowedOrigins = append(allowedOrigins, u.Scheme+"://"+u.Host)
				} else {
					allowedOrigins = append(allowedOrigins, p)
				}
			}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// If no allowedOrigins configured, allow any origin
			if len(allowedOrigins) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				allowed := false
				for _, ao := range allowedOrigins {
					if ao == origin {
						allowed = true
						break
					}
				}
				if allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					// Allow credentials (cookies/auth) for allowed origins
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			// If the browser sent requested headers in preflight, echo them back
			// which allows custom headers like `x-health-check`.
			reqHeaders := r.Header.Get("Access-Control-Request-Headers")
			if reqHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
			} else {
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Authorization, X-Requested-With, X-Health-Check")
			}

			if r.Method == http.MethodOptions {
				// For preflight, ensure origin is allowed
				if len(allowedOrigins) > 0 && origin != "" {
					allowed := false
					for _, ao := range allowedOrigins {
						if ao == origin {
							allowed = true
							break
						}
					}
					if !allowed {
						http.Error(w, "origin not allowed", http.StatusForbidden)
						return
					}
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
