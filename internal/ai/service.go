package ai

import (
	"encoding/json"
	"fmt"
	"places_api/internal/config"
	"places_api/internal/logger"
	"places_api/internal/types"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// Service represents the AI service for interacting with OpenRouter
type Service struct {
	client       *resty.Client
	apiKey       string
	model        string
	baseURL      string
	queryBuilder *QueryBuilder
	logger       *logger.Logger
}

// OpenRouterRequest represents the request structure for OpenRouter API
type OpenRouterRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Message represents a chat message in the OpenRouter API
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenRouterResponse represents the response structure from OpenRouter API
type OpenRouterResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a choice in the OpenRouter response
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewService creates a new AI service instance
func NewService(cfg *config.Config) *Service {
	client := resty.New()
	client.SetTimeout(180 * time.Second) // Increased to 180s for free tier models
	client.SetRetryCount(3)
	client.SetRetryWaitTime(2 * time.Second)
	client.SetRetryMaxWaitTime(10 * time.Second)

	return &Service{
		client:       client,
		apiKey:       cfg.AI.OpenRouterAPIKey,
		model:        cfg.AI.Model,
		baseURL:      "https://openrouter.ai/api/v1",
		queryBuilder: NewQueryBuilder(),
		logger:       logger.WithComponent("ai-service"),
	}
}

// GetTopAttractions fetches top attractions for a given area using AI
func (s *Service) GetTopAttractions(area *types.Area) (*types.AttractionResponse, error) {
	start := time.Now()
	areaLogger := s.logger.WithArea(area.AreaKey)

	areaLogger.Info().Str("area_name", area.Name).Msg("Starting AI attractions request")

	if s.apiKey == "" {
		areaLogger.Error().Msg("OpenRouter API key is not configured")
		return nil, fmt.Errorf("OpenRouter API key is not configured")
	}

	// Create the prompt for getting top attractions
	prompt := s.queryBuilder.CreateAttractionsPrompt(area)
	areaLogger.Debug().Int("prompt_length", len(prompt)).Msg("Generated AI prompt")

	// Prepare the request
	request := OpenRouterRequest{
		Model: s.model,
		Messages: []Message{
			{
				Role:    "system",
				Content: s.queryBuilder.GetAttractionsSystemMessage(),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	// Make the API call
	areaLogger.Debug().Str("model", s.model).Msg("Making OpenRouter API request")
	apiStart := time.Now()

	resp, err := s.client.R().
		SetHeader("Authorization", "Bearer "+s.apiKey).
		SetHeader("Content-Type", "application/json").
		SetHeader("HTTP-Referer", "https://places-api.com").
		SetHeader("X-Title", "Places API").
		SetBody(request).
		Post(s.baseURL + "/chat/completions")

	apiDuration := time.Since(apiStart)

	if err != nil {
		areaLogger.Error().
			Err(err).
			Dur("duration", apiDuration).
			Str("model", s.model).
			Msg("OpenRouter API request failed")

		// Check if it's a timeout error
		if apiDuration >= 120*time.Second {
			return nil, fmt.Errorf("API request timed out after %v - consider using a faster model or increasing timeout: %w", apiDuration, err)
		}
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}

	areaLogger.LogAPICall("openrouter", "/chat/completions", "POST", resp.StatusCode(), apiDuration)

	if resp.StatusCode() != 200 {
		areaLogger.Error().Int("status_code", resp.StatusCode()).Str("response", resp.String()).Msg("OpenRouter API returned error")
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode(), resp.String())
	}

	// Parse the response
	var openRouterResp OpenRouterResponse
	if err := json.Unmarshal(resp.Body(), &openRouterResp); err != nil {
		areaLogger.Error().Err(err).Msg("Failed to parse OpenRouter API response")
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(openRouterResp.Choices) == 0 {
		areaLogger.Error().Msg("No choices returned from OpenRouter API")
		return nil, fmt.Errorf("no choices returned from API")
	}

	// Log token usage
	areaLogger.Info().
		Int("prompt_tokens", openRouterResp.Usage.PromptTokens).
		Int("completion_tokens", openRouterResp.Usage.CompletionTokens).
		Int("total_tokens", openRouterResp.Usage.TotalTokens).
		Msg("OpenRouter API token usage")

	// Extract the content from the first choice
	content := openRouterResp.Choices[0].Message.Content
	areaLogger.Debug().Int("response_length", len(content)).Msg("Received AI response")

	// Strip markdown code fences if present (AI sometimes returns JSON wrapped in ```)
	content = stripMarkdownCodeFences(content)

	// Parse the JSON response
	var attractionResp types.AttractionResponse
	if err := json.Unmarshal([]byte(content), &attractionResp); err != nil {
		// Log the raw content for debugging
		areaLogger.Error().Err(err).Str("raw_content", content).Msg("Failed to parse attraction response JSON")
		return nil, fmt.Errorf("failed to parse attraction response: %w", err)
	}

	// Log success metrics
	totalDuration := time.Since(start)
	totalAttractions := len(attractionResp.Attractions) + len(attractionResp.Restaurants) +
		len(attractionResp.Cafes) + len(attractionResp.Bars) + len(attractionResp.Hotels)

	areaLogger.Info().
		Int("attractions_count", len(attractionResp.Attractions)).
		Int("restaurants_count", len(attractionResp.Restaurants)).
		Int("cafes_count", len(attractionResp.Cafes)).
		Int("bars_count", len(attractionResp.Bars)).
		Int("hotels_count", len(attractionResp.Hotels)).
		Int("total_items", totalAttractions).
		Float64("total_duration", totalDuration.Seconds()).
		Msg("Successfully retrieved attractions from AI")

	return &attractionResp, nil
}

// stripMarkdownCodeFences removes markdown code fence markers (```) from the content
// This handles cases where AI models return JSON wrapped in markdown code blocks
func stripMarkdownCodeFences(content string) string {
	content = strings.TrimSpace(content)

	// Remove leading triple backticks and optional language identifier
	if strings.HasPrefix(content, "```") {
		// Find the first newline after the opening ```
		newlineIndex := strings.Index(content, "\n")
		if newlineIndex != -1 {
			// Remove everything up to and including the newline
			content = content[newlineIndex+1:]
		} else {
			// No newline, remove the ``` and trim any whitespace/language identifier
			content = strings.TrimPrefix(content, "```")
			content = strings.TrimSpace(content)
		}
	}

	// Remove trailing triple backticks
	content = strings.TrimSpace(content)
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	return content
}
