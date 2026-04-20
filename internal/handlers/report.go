package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/farigab/bragdoc/internal/middleware"
	"github.com/farigab/bragdoc/internal/usecase"
)

// RegisterReportRoutes registers report endpoints.
// The router r must already have the Auth middleware applied.
func RegisterReportRoutes(r chi.Router, reportSvc *usecase.ReportService) {
	r.Post("/api/reports/ai-summary/custom", func(w http.ResponseWriter, req *http.Request) {
		userLogin, ok := middleware.UserLoginFromContext(req.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		req.Body = http.MaxBytesReader(w, req.Body, maxBodyBytes)
		var body struct {
			ReportType   string   `json:"reportType"`
			Category     string   `json:"category"`
			StartDate    string   `json:"startDate"`
			EndDate      string   `json:"endDate"`
			UserPrompt   string   `json:"userPrompt"`
			Repositories []string `json:"repositories"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		var sdt, edt time.Time
		if body.StartDate != "" {
			sdt, _ = time.Parse("2006-01-02", body.StartDate)
		}
		if body.EndDate != "" {
			edt, _ = time.Parse("2006-01-02", body.EndDate)
		}

		out, err := reportSvc.Generate(usecase.GenerateReportInput{
			UserLogin:    userLogin,
			ReportType:   body.ReportType,
			Category:     body.Category,
			StartDate:    sdt,
			EndDate:      edt,
			UserPrompt:   body.UserPrompt,
			Repositories: body.Repositories,
		})
		if err != nil {
			log.Printf("report generation error: %v", err)
			if strings.Contains(err.Error(), "user not found") {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			http.Error(w, "ai generation failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"report": out})
	})
}
