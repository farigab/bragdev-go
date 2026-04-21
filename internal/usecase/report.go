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
	fetcherFactory integration.CommitFetcherFactory
	ai             integration.AIReportGenerator
}

func NewReportService(
	userRepo repository.UserRepository,
	fetcherFactory integration.CommitFetcherFactory,
	ai integration.AIReportGenerator,
) *ReportService {
	return &ReportService{
		userRepo:       userRepo,
		fetcherFactory: fetcherFactory,
		ai:             ai,
	}
}

// Generate collects GitHub commit data, builds an AI prompt, and returns the
// generated report text.
func (s *ReportService) Generate(in GenerateReportInput) (string, error) {
	filtered, err := s.collectCommitData(in)
	if err != nil {
		return "", err
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

// collectCommitData fetches commits for every non-empty repository. Repos that
// error or return no commits are silently skipped so the caller always gets the
// best available data. Returns an empty slice when no repositories are provided.
func (s *ReportService) collectCommitData(in GenerateReportInput) ([]any, error) {
	repos := nonEmptyRepos(in.Repositories)
	if len(repos) == 0 {
		return []any{}, nil
	}

	u, err := s.userRepo.FindByLogin(in.UserLogin)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	fetcher := s.fetcherFactory.New(strings.TrimSpace(u.GitHubAccessToken))
	filtered := make([]any, 0, len(repos))

	for _, repo := range repos {
		commits, err := fetcher.ListCommitMessages(repo, in.UserLogin, in.StartDate, in.EndDate)
		if err != nil || len(commits) == 0 {
			continue // partial failure: skip repo, caller sees available data
		}
		filtered = append(filtered, map[string]any{
			"repo":    repo,
			"commits": commits,
		})
	}

	return filtered, nil
}

// nonEmptyRepos returns only the non-blank entries from the input slice.
func nonEmptyRepos(repos []string) []string {
	result := make([]string, 0, len(repos))
	for _, r := range repos {
		if strings.TrimSpace(r) != "" {
			result = append(result, r)
		}
	}
	return result
}
