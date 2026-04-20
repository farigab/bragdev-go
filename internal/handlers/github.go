package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/farigab/bragdoc/internal/integration"
	"github.com/farigab/bragdoc/internal/middleware"
	"github.com/farigab/bragdoc/internal/repository"
)

// RegisterGitHubRoutes registers GitHub-related endpoints.
// The router r must already have the Auth middleware applied.
func RegisterGitHubRoutes(r chi.Router, userRepo repository.UserRepository) {
	r.Post("/api/github/import/repositories", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		u, err := userRepo.FindByLogin(userLogin)
		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if strings.TrimSpace(u.GitHubAccessToken) == "" {
			http.Error(w, "github token not found", http.StatusBadRequest)
			return
		}

		client := integration.NewGitHubClient(u.GitHubAccessToken)
		repos, err := client.ListRepositories()
		if err != nil {
			log.Printf("list repos error: %v", err)
			http.Error(w, "failed to list repositories", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(repos)
	})

	r.Post("/api/github/import", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		u, err := userRepo.FindByLogin(userLogin)
		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		if strings.TrimSpace(u.GitHubAccessToken) == "" {
			http.Error(w, "github token not found", http.StatusBadRequest)
			return
		}

		req.Body = http.MaxBytesReader(w, req.Body, maxBodyBytes)
		var body struct {
			Repositories []string `json:"repositories"`
			DataInicio   string   `json:"dataInicio"`
			DataFim      string   `json:"dataFim"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		client := integration.NewGitHubClient(u.GitHubAccessToken)
		// if repositories not provided, list user's repos
		repos := body.Repositories
		if len(repos) == 0 {
			repos, err = client.ListRepositories()
			if err != nil {
				log.Printf("list repos error: %v", err)
				http.Error(w, "failed to list repositories", http.StatusInternalServerError)
				return
			}
		}

		var since, until time.Time
		if body.DataInicio != "" {
			since, _ = time.Parse("2006-01-02", body.DataInicio)
		}
		if body.DataFim != "" {
			until, _ = time.Parse("2006-01-02", body.DataFim)
		}

		details := map[string]int{}
		total := 0
		for _, repoFull := range repos {
			// repoFull expected as owner/name
			c, err := client.CountCommits(repoFull, userLogin, since, until)
			if err != nil {
				log.Printf("count commits %s: %v", repoFull, err)
				// continue on errors per-repo
				continue
			}
			details[repoFull] = c
			total += c
		}

		resp := map[string]any{
			"repositories": repos,
			"totalCommits": total,
			"details":      details,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}
