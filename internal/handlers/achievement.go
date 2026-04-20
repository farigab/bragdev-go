package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/farigab/bragdoc/internal/domain"
	"github.com/farigab/bragdoc/internal/middleware"
	"github.com/farigab/bragdoc/internal/repository"
)

// maxBodyBytes is the maximum accepted request body size (1 MB).
const maxBodyBytes = 1 << 20

// RegisterAchievementRoutes sets up achievement CRUD endpoints.
// The router r must already have the Auth middleware applied.
func RegisterAchievementRoutes(r chi.Router, achRepo repository.AchievementRepository) {
	r.Post("/api/achievements", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		req.Body = http.MaxBytesReader(w, req.Body, maxBodyBytes)
		var payload struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Category    string `json:"category"`
			Date        string `json:"date"` // YYYY-MM-DD
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		var dt time.Time
		var err error
		if payload.Date != "" {
			dt, err = time.Parse("2006-01-02", payload.Date)
			if err != nil {
				http.Error(w, "invalid date format, expected YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		} else {
			dt = time.Now()
		}

		a := domain.NewAchievement(payload.Title, payload.Description, payload.Category, dt, userLogin)
		saved, err := achRepo.Save(a)
		if err != nil {
			log.Printf("error saving achievement: %v", err)
			http.Error(w, "failed to save", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(saved)
	})

	r.Put("/api/achievements/{id}", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(req, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		existing, err := achRepo.FindByID(id, userLogin)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		req.Body = http.MaxBytesReader(w, req.Body, maxBodyBytes)
		var payload struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Category    string `json:"category"`
			Date        string `json:"date"` // YYYY-MM-DD
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		if payload.Title != "" {
			existing.Title = payload.Title
		}
		if payload.Description != "" {
			existing.Description = payload.Description
		}
		if payload.Category != "" {
			existing.Category = payload.Category
		}
		if payload.Date != "" {
			dt, err := time.Parse("2006-01-02", payload.Date)
			if err == nil {
				existing.Date = dt
			}
		}

		saved, err := achRepo.Save(existing)
		if err != nil {
			http.Error(w, "failed to update", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(saved)
	})

	r.Delete("/api/achievements/{id}", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(req, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		if err := achRepo.Delete(id, userLogin); err != nil {
			http.Error(w, "failed to delete", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	r.Get("/api/achievements/{id}", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(req, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		a, err := achRepo.FindByID(id, userLogin)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(a)
	})

	r.Get("/api/achievements", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		list, err := achRepo.FindByUser(userLogin)
		if err != nil {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	})

	r.Get("/api/achievements/category/{category}", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		cat := chi.URLParam(req, "category")
		list, err := achRepo.FindByUser(userLogin)
		if err != nil {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		var filtered []*domain.Achievement
		for _, a := range list {
			if strings.EqualFold(a.Category, cat) {
				filtered = append(filtered, a)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(filtered)
	})

	r.Get("/api/achievements/date-range", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		start := req.URL.Query().Get("startDate")
		end := req.URL.Query().Get("endDate")
		if start == "" || end == "" {
			http.Error(w, "missing dates", http.StatusBadRequest)
			return
		}
		sdt, err := time.Parse("2006-01-02", start)
		if err != nil {
			http.Error(w, "invalid startDate", http.StatusBadRequest)
			return
		}
		edt, err := time.Parse("2006-01-02", end)
		if err != nil {
			http.Error(w, "invalid endDate", http.StatusBadRequest)
			return
		}

		list, err := achRepo.FindByUser(userLogin)
		if err != nil {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		var filtered []*domain.Achievement
		for _, a := range list {
			if (a.Date.Equal(sdt) || a.Date.After(sdt)) && (a.Date.Equal(edt) || a.Date.Before(edt)) {
				filtered = append(filtered, a)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(filtered)
	})

	r.Get("/api/achievements/search", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		keyword := strings.ToLower(req.URL.Query().Get("keyword"))
		if keyword == "" {
			http.Error(w, "keyword missing", http.StatusBadRequest)
			return
		}

		list, err := achRepo.FindByUser(userLogin)
		if err != nil {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		var filtered []*domain.Achievement
		for _, a := range list {
			if strings.Contains(strings.ToLower(a.Title), keyword) {
				filtered = append(filtered, a)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(filtered)
	})
}
