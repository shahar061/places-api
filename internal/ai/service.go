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

// GetAirportMajorCity determines the major city for an airport using AI
func (s *Service) GetAirportMajorCity(airportName, regionName string) (*types.AirportMajorCityResponse, error) {
	start := time.Now()
	reqLogger := &logger.Logger{
		Logger: s.logger.With().
			Str("airport_name", airportName).
			Str("region_name", regionName).
			Logger(),
	}

	reqLogger.Info().Msg("Starting AI airport major city request")

	if s.apiKey == "" {
		reqLogger.Error().Msg("OpenRouter API key is not configured")
		return nil, fmt.Errorf("OpenRouter API key is not configured")
	}

	// Create the prompt for determining major city
	prompt := s.queryBuilder.CreateAirportMajorCityPrompt(airportName, regionName)
	reqLogger.Debug().Int("prompt_length", len(prompt)).Msg("Generated AI prompt")

	// Prepare the request
	request := OpenRouterRequest{
		Model: s.model,
		Messages: []Message{
			{
				Role:    "system",
				Content: s.queryBuilder.GetAirportMajorCitySystemMessage(),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	// Make the API call
	reqLogger.Debug().Str("model", s.model).Msg("Making OpenRouter API request")
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
		reqLogger.Error().
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

	reqLogger.LogAPICall("openrouter", "/chat/completions", "POST", resp.StatusCode(), apiDuration)

	if resp.StatusCode() != 200 {
		reqLogger.Error().Int("status_code", resp.StatusCode()).Str("response", resp.String()).Msg("OpenRouter API returned error")
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode(), resp.String())
	}

	// Parse the response
	var openRouterResp OpenRouterResponse
	if err := json.Unmarshal(resp.Body(), &openRouterResp); err != nil {
		reqLogger.Error().Err(err).Msg("Failed to parse OpenRouter API response")
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(openRouterResp.Choices) == 0 {
		reqLogger.Error().Msg("No choices returned from OpenRouter API")
		return nil, fmt.Errorf("no choices returned from API")
	}

	// Log token usage
	reqLogger.Info().
		Int("prompt_tokens", openRouterResp.Usage.PromptTokens).
		Int("completion_tokens", openRouterResp.Usage.CompletionTokens).
		Int("total_tokens", openRouterResp.Usage.TotalTokens).
		Msg("OpenRouter API token usage")

	// Extract the content from the first choice
	content := openRouterResp.Choices[0].Message.Content
	reqLogger.Debug().Int("response_length", len(content)).Msg("Received AI response")

	// Strip markdown code fences if present (AI sometimes returns JSON wrapped in ```)
	content = stripMarkdownCodeFences(content)

	// Parse the JSON response
	var majorCityResp types.AirportMajorCityResponse
	if err := json.Unmarshal([]byte(content), &majorCityResp); err != nil {
		// Log the raw content for debugging
		reqLogger.Error().Err(err).Str("raw_content", content).Msg("Failed to parse airport major city response JSON")
		return nil, fmt.Errorf("failed to parse airport major city response: %w", err)
	}

	// Log success metrics
	totalDuration := time.Since(start)

	reqLogger.Info().
		Str("major_city", majorCityResp.MajorCity).
		Str("country", majorCityResp.Country).
		Float64("confidence", majorCityResp.Confidence).
		Float64("total_duration", totalDuration.Seconds()).
		Msg("Successfully determined airport major city from AI")

	return &majorCityResp, nil
}

// GenerateTripItinerary generates a complete trip itinerary using AI
func (s *Service) GenerateTripItinerary(trip *types.Trip, preferences *types.ItineraryPreferences) (*types.TripItineraryResponse, error) {
	start := time.Now()

	s.logger.Info().
		Str("trip_id", trip.ID).
		Str("trip_name", trip.Name).
		Msg("Starting AI itinerary generation")

	if s.apiKey == "" {
		return nil, fmt.Errorf("OpenRouter API key is not configured")
	}

	// Create the prompt
	prompt := s.queryBuilder.CreateTripItineraryPrompt(trip, preferences)
	s.logger.Debug().Int("prompt_length", len(prompt)).Msg("Generated AI prompt")

	// Prepare the request
	request := OpenRouterRequest{
		Model: s.model,
		Messages: []Message{
			{
				Role:    "system",
				Content: s.queryBuilder.GetTripItinerarySystemMessage(),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	// Make the API call
	s.logger.Debug().Str("model", s.model).Msg("Making OpenRouter API request")
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
		s.logger.Error().
			Err(err).
			Dur("duration", apiDuration).
			Msg("OpenRouter API request failed")
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}

	s.logger.Info().
		Int("status_code", resp.StatusCode()).
		Dur("duration", apiDuration).
		Msg("OpenRouter API call completed")

	if resp.StatusCode() != 200 {
		s.logger.Error().
			Int("status_code", resp.StatusCode()).
			Str("response", resp.String()).
			Msg("OpenRouter API returned error")
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode(), resp.String())
	}

	// Parse the response
	var openRouterResp OpenRouterResponse
	if err := json.Unmarshal(resp.Body(), &openRouterResp); err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse OpenRouter API response")
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(openRouterResp.Choices) == 0 {
		s.logger.Error().Msg("No choices returned from OpenRouter API")
		return nil, fmt.Errorf("no choices returned from API")
	}

	// Log token usage
	s.logger.Info().
		Int("prompt_tokens", openRouterResp.Usage.PromptTokens).
		Int("completion_tokens", openRouterResp.Usage.CompletionTokens).
		Int("total_tokens", openRouterResp.Usage.TotalTokens).
		Msg("OpenRouter API token usage")

	// Extract the content
	content := openRouterResp.Choices[0].Message.Content
	s.logger.Debug().Int("response_length", len(content)).Msg("Received AI response")

	// Strip markdown code fences if present
	content = stripMarkdownCodeFences(content)

	// Parse the JSON response
	var itineraryResp types.TripItineraryResponse
	if err := json.Unmarshal([]byte(content), &itineraryResp); err != nil {
		s.logger.Error().
			Err(err).
			Str("raw_content", content).
			Msg("Failed to parse itinerary response JSON")
		return nil, fmt.Errorf("failed to parse itinerary response: %w", err)
	}

	// Log success metrics
	totalDuration := time.Since(start)
	totalActivities := 0
	for _, day := range itineraryResp.Itinerary.Days {
		totalActivities += len(day.Activities)
	}

	s.logger.Info().
		Int("days_count", len(itineraryResp.Itinerary.Days)).
		Int("total_activities", totalActivities).
		Float64("total_duration", totalDuration.Seconds()).
		Msg("Successfully generated trip itinerary")

	return &itineraryResp, nil
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
