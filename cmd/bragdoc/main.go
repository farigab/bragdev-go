// Package main is the server entrypoint for the bragdoc application.
package main

import (
	"fmt"
	"net"
	"net/http"
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
	"github.com/farigab/bragdoc/internal/logger"
	appMiddleware "github.com/farigab/bragdoc/internal/middleware"
	"github.com/farigab/bragdoc/internal/repository"
	"github.com/farigab/bragdoc/internal/security"
	"github.com/farigab/bragdoc/internal/usecase"
)

func main() {
	cfg := config.Load()

	// Initialize logger early so startup messages are visible and consistent
	logger.Init(cfg.LogLevel)

	db, err := sqlx.Connect("postgres", cfg.DBUrl)
	if err != nil {
		logger.Errorf("failed to connect to db: %v", err)
		os.Exit(1)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			logger.Errorf("failed to close db: %v", cerr)
		}
	}()

	// Optionally run migrations if MIGRATE=true
	if strings.ToLower(os.Getenv("MIGRATE")) == "true" {
		if err := runMigrations(db, "db/migrations"); err != nil {
			logger.Errorf("migrations failed: %v", err)
			os.Exit(1)
		}
	}

	r := chi.NewRouter()

	// CORS middleware
	r.Use(appMiddleware.CORSMiddleware(cfg))

	// Request logging + panic recovery
	r.Use(appMiddleware.RequestLogger)

	// health
	r.Get("/api/health", handlers.HealthHandler)

	// wiring
	userRepo := repository.NewPostgresUserRepo(db)
	refreshRepo := repository.NewPostgresRefreshTokenRepo(db)

	jwtSvc := security.NewJWTService(cfg.JwtSecret, 900) // 15 minutes
	oauthSvc := integration.NewGitHubOAuthService(cfg.GitHubClientID, cfg.GitHubClientSecret)
	geminiClient := integration.NewGeminiClient(cfg.GeminiAPIKey, cfg.GeminiAPIURL, cfg.GeminiModel)
	fetcherFactory := integration.GitHubClientFactory{}
	reportSvc := usecase.NewReportService(userRepo, fetcherFactory, geminiClient)

	// Public auth routes (OAuth flow + token refresh)
	handlers.RegisterAuthRoutes(r, cfg, oauthSvc, jwtSvc, userRepo, refreshRepo)

	// Protected routes — require a valid JWT cookie
	r.Group(func(r chi.Router) {
		r.Use(appMiddleware.AuthWithRefresh(cfg, jwtSvc, userRepo, refreshRepo))
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
				logger.Errorw("failed cleaning expired refresh tokens", "err", err)
			}
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Infow("starting server", "port", port)
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Errorf("failed to bind port: %v", err)
		os.Exit(1)
	}

	logger.Infow("server started", "port", port)
	if err := http.Serve(ln, r); err != nil {
		logger.Errorf("server error: %v", err)
		os.Exit(1)
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
		logger.Infow("applied migration", "migration", n)
	}
	return nil
}
