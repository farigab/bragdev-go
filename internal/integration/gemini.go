package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AIReportGenerator generates AI reports from text prompts.
type AIReportGenerator interface {
	GenerateReport(ctx context.Context, prompt string) (string, error)
}

// GenerationConfig holds tunable parameters for the Gemini generation API.
type GenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// GeminiClient is a tolerant client for Google Gemini-like endpoints.
type GeminiClient struct {
	client           *http.Client
	apiKey           string
	apiURL           string
	model            string
	generationConfig GenerationConfig
}

// DefaultGenerationConfig returns sensible defaults for report generation.
func DefaultGenerationConfig() GenerationConfig {
	return GenerationConfig{
		Temperature:     0.4,
		TopP:            0.95,
		TopK:            40,
		MaxOutputTokens: 4096,
	}
}

// NewGeminiClient creates a new GeminiClient configured with apiKey and model.
func NewGeminiClient(apiKey, apiURL, model string) *GeminiClient {
	if apiURL == "" {
		apiURL = "https://generativelanguage.googleapis.com/v1"
	}
	return &GeminiClient{
		client:           &http.Client{Timeout: 30 * time.Second},
		apiKey:           apiKey,
		apiURL:           apiURL,
		model:            model,
		generationConfig: DefaultGenerationConfig(),
	}
}

// WithGenerationConfig returns a copy of the client with the given config.
func (g *GeminiClient) WithGenerationConfig(cfg GenerationConfig) *GeminiClient {
	cp := *g
	cp.generationConfig = cfg
	return &cp
}

// GenerateReport sends a prompt and extracts text from multiple response shapes.
func (g *GeminiClient) GenerateReport(ctx context.Context, prompt string) (string, error) {
	if g.apiKey == "" {
		return "", errors.New("gemini api key not configured")
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.apiURL, g.model, g.apiKey)

	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": g.generationConfig,
	}

	jb, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(jb),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer closeBody(resp.Body)

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, string(respBody))
	}

	// Estrutura oficial da resposta
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("no response text")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}
