package usecase

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/farigab/bragdoc/internal/integration"
	"github.com/farigab/bragdoc/internal/report"
	"github.com/farigab/bragdoc/internal/repository"
)

// GenerateReportInput holds the parameters for report generation.
type GenerateReportInput struct {
	UserLogin    string
	ReportType   string
	Category     string
	StartDate    time.Time
	EndDate      time.Time
	UserPrompt   string
	Repositories []string
}

// ReportService encapsulates report generation business logic, keeping it
// decoupled from HTTP transport concerns.
type ReportService struct {
	userRepo       repository.UserRepository
	achRepo        repository.AchievementRepository
	fetcherFactory integration.CommitFetcherFactory
	ai             integration.AIReportGenerator
}

func NewReportService(
	userRepo repository.UserRepository,
	achRepo repository.AchievementRepository,
	fetcherFactory integration.CommitFetcherFactory,
	ai integration.AIReportGenerator,
) *ReportService {
	return &ReportService{
		userRepo:       userRepo,
		achRepo:        achRepo,
		fetcherFactory: fetcherFactory,
		ai:             ai,
	}
}

// Generate collects data from either GitHub commits or local achievements,
// builds an AI prompt, and returns the generated report text.
func (s *ReportService) Generate(in GenerateReportInput) (string, error) {
	filtered := make([]any, 0)

	hasRepos := false
	for _, r := range in.Repositories {
		if strings.TrimSpace(r) != "" {
			hasRepos = true
			break
		}
	}

	if hasRepos {
		u, err := s.userRepo.FindByLogin(in.UserLogin)
		if err != nil {
			return "", fmt.Errorf("user not found: %w", err)
		}

		fetcher := s.fetcherFactory.New(strings.TrimSpace(u.GitHubAccessToken))

		for _, repoFull := range in.Repositories {
			if strings.TrimSpace(repoFull) == "" {
				continue
			}
			commits, err := fetcher.ListCommitMessages(repoFull, in.UserLogin, in.StartDate, in.EndDate)
			if err != nil {
				// partial failure: skip repo, caller sees available data
				continue
			}
			if len(commits) == 0 {
				continue
			}
			filtered = append(filtered, map[string]any{
				"repo":    repoFull,
				"commits": commits,
			})
		}
	} else {
		list, err := s.achRepo.FindByUser(in.UserLogin)
		if err != nil {
			return "", fmt.Errorf("failed fetching achievements: %w", err)
		}
		for _, a := range list {
			if in.Category != "" && a.Category != in.Category {
				continue
			}
			if !in.StartDate.IsZero() && a.Date.Before(in.StartDate) {
				continue
			}
			if !in.EndDate.IsZero() && a.Date.After(in.EndDate) {
				continue
			}
			filtered = append(filtered, a)
		}
	}

	achievementsDataBytes, err := json.Marshal(filtered)
	if err != nil {
		return "", fmt.Errorf("failed to serialize data: %w", err)
	}

	prompt := report.BuildPrompt(string(achievementsDataBytes), in.ReportType)
	if in.UserPrompt != "" {
		prompt = in.UserPrompt + "\n\n" + prompt
	}

	return s.ai.GenerateReport(prompt)
}
