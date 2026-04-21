package main

import (
	"fmt"
	"log"
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
	r.Use(appMiddleware.CORSMiddleware(cfg))

	// health
	r.Get("/api/health", handlers.HealthHandler)

	// wiring
	userRepo := repository.NewPostgresUserRepo(db)
	refreshRepo := repository.NewPostgresRefreshTokenRepo(db)

	jwtSvc := security.NewJWTService(cfg.JwtSecret, 900) // 15 minutes
	oauthSvc := integration.NewGitHubOAuthService(cfg.GitHubClientID, cfg.GitHubClientSecret)
	geminiClient := integration.NewGeminiClient(cfg.GeminiApiKey, cfg.GeminiApiUrl, cfg.GeminiModel)
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
