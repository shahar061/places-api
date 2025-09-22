package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"places_api/internal/config"
	"places_api/internal/types"
	"time"

	"github.com/google/uuid"
)

// SupabaseService interface for saving places
type SupabaseService interface {
	SavePlaces(places []types.Place, areaKey string) error
	GetTopPlaces(areaKey string, categories string, limit, offset int) ([]types.Place, error)
}

// OverpassService interface for getting exact locations
type OverpassService interface {
	FixLocation(placeName, cityName string, approxLat, approxLon float64) (float64, float64)
	FindPlaceLocation(placeName, cityName string) (*types.Place, error)
}

// NominatimService interface for geocoding
type NominatimService interface {
	GeocodeStructured(queryLoc types.QueryLocation) (*types.Area, error)
}

// AIPlanner service handles AI-powered travel planning using OpenRouter API
type AIPlannerService struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	model      string
	supabase   SupabaseService
	overpass   OverpassService
	nominatim  NominatimService
}

// OpenRouterRequest represents the request structure for OpenRouter API
type OpenRouterRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenRouterMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
	Stream      bool                `json:"stream"`
}

// OpenRouterMessage represents a message in the conversation
type OpenRouterMessage struct {
	Role    string `json:"role"` // "system", "user", or "assistant"
	Content string `json:"content"`
}

// OpenRouterResponse represents the response from OpenRouter API
type OpenRouterResponse struct {
	ID      string             `json:"id"`
	Choices []OpenRouterChoice `json:"choices"`
	Usage   OpenRouterUsage    `json:"usage"`
	Error   *OpenRouterError   `json:"error,omitempty"`
}

// OpenRouterChoice represents a choice in the response
type OpenRouterChoice struct {
	Index        int               `json:"index"`
	Message      OpenRouterMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

// OpenRouterUsage represents token usage information
type OpenRouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenRouterError represents an error from OpenRouter API
type OpenRouterError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// NewAIPlannerService creates a new AI Planner service
func NewAIPlannerService(cfg *config.AIConfig, supabase SupabaseService, overpass OverpassService, nominatim NominatimService) (*AIPlannerService, error) {
	if cfg.OpenRouterAPIKey == "" {
		return nil, fmt.Errorf("OpenRouter API key is required")
	}

	return &AIPlannerService{
		apiKey:  cfg.OpenRouterAPIKey,
		baseURL: "https://openrouter.ai/api/v1",
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for AI responses
		},
		model:     cfg.Model, // Default to a good model if not specified
		supabase:  supabase,
		overpass:  overpass,
		nominatim: nominatim,
	}, nil
}

// GeneratePlan creates a travel plan based on the request
func (ai *AIPlannerService) GeneratePlan(req *types.PlanRequest) (*types.PlanResponse, error) {
	// Build the system prompt
	systemPrompt := `You are a travel planning expert. Create detailed, practical travel itineraries in JSON format.
Always respond with valid JSON matching this structure:
{
  "title": "Trip title",
  "description": "Brief overview", 
  "days": [
    {
      "day": 1,
      "title": "Day title",
      "description": "Day overview",
      "activities": [
        {
          "time": "09:00",
          "title": "Activity name",
          "description": "Detailed description",
          "location": "Specific location",
          "category": "sightseeing|food|transport|shopping|entertainment",
          "duration": "2 hours",
          "cost": "€15-25"
        }
      ]
    }
  ],
  "tips": ["Helpful travel tips"]
}`

	// Build the user prompt
	userPrompt := fmt.Sprintf(`Create a %d-day travel plan for %s.`, req.Duration, req.Area)

	if len(req.Interests) > 0 {
		userPrompt += fmt.Sprintf(" Interests: %v.", req.Interests)
	}
	if req.Budget != "" {
		userPrompt += fmt.Sprintf(" Budget: %s.", req.Budget)
	}
	if req.TravelStyle != "" {
		userPrompt += fmt.Sprintf(" Travel style: %s.", req.TravelStyle)
	}

	userPrompt += " Include specific locations, realistic timing, and practical tips. Focus on must-see attractions, local cuisine, and authentic experiences."

	// Make the API request
	content, err := ai.makeAPIRequest(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan: %v", err)
	}

	// Parse the AI response
	var planData struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Days        []types.PlanDay `json:"days"`
		Tips        []string        `json:"tips"`
	}

	if err := json.Unmarshal([]byte(content), &planData); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %v", err)
	}

	// Create the response
	response := &types.PlanResponse{
		ID:          uuid.New().String(),
		Area:        req.Area,
		Duration:    req.Duration,
		Title:       planData.Title,
		Description: planData.Description,
		Days:        planData.Days,
		Tips:        planData.Tips,
		GeneratedAt: time.Now(),
	}

	return response, nil
}

// ChatTravel handles travel-related questions with context
func (ai *AIPlannerService) ChatTravel(req *types.ChatRequest) (*types.ChatResponse, error) {
	systemPrompt := `You are a knowledgeable travel advisor. Provide helpful, practical travel advice.
Be concise but informative. Focus on actionable recommendations.
If appropriate, suggest 2-3 related follow-up questions the user might find helpful.`

	userPrompt := fmt.Sprintf("Location context: %s\n\nQuestion: %s", req.Area, req.Question)
	if req.Context != "" {
		userPrompt += fmt.Sprintf("\n\nAdditional context: %s", req.Context)
	}

	content, err := ai.makeAPIRequest(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to get travel advice: %v", err)
	}

	response := &types.ChatResponse{
		Answer:      content,
		GeneratedAt: time.Now(),
	}

	return response, nil
}

// makeAPIRequest makes a request to the OpenRouter API
func (ai *AIPlannerService) makeAPIRequest(systemPrompt, userPrompt string) (string, error) {
	// Validate required fields
	if ai.model == "" {
		return "", fmt.Errorf("model is required but empty")
	}
	if ai.apiKey == "" {
		return "", fmt.Errorf("API key is required but empty")
	}

	// Prepare the request
	reqData := OpenRouterRequest{
		Model: ai.model,
		Messages: []OpenRouterMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		MaxTokens:   4000,
		Temperature: 0.7,
		TopP:        0.9,
		Stream:      false,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Debug: log the request being sent
	fmt.Printf("OpenRouter Request: %s\n", string(jsonData))

	// Create HTTP request
	req, err := http.NewRequest("POST", ai.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.apiKey)
	req.Header.Set("HTTP-Referer", "https://places-api.example.com") // Required by OpenRouter
	req.Header.Set("X-Title", "Places API")                          // Required by OpenRouter

	// Make the request
	resp, err := ai.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error response body for debugging
		var errorBody bytes.Buffer
		errorBody.ReadFrom(resp.Body)
		return "", fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, errorBody.String())
	}

	// Parse the response
	var apiResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("OpenRouter API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned from OpenRouter API")
	}

	return apiResp.Choices[0].Message.Content, nil
}

// GetAreaTopAttractions gets the top 5 attractions for each category (attraction, restaurant, cafe, bar, hotel) for an area and saves them to the database
func (ai *AIPlannerService) GetAreaTopAttractions(city string, areaKey string) error {
	// First, we need to check if the places already exist in the database
	if ai.supabase != nil {
		// the key prefix is the area key and the rest of the key is the place name
		places, err := ai.supabase.GetTopPlaces(areaKey, "attraction,restaurant,cafe,bar,hotel", 5, 0)
		if err != nil {
			return fmt.Errorf("failed to get top attractions: %v", err)
		}

		if len(places) > 0 {
			return nil
		}
	}

	systemPrompt := getTopAttractionsSystemPrompt

	userPrompt := fmt.Sprintf(getTopAttractionsQuery, city, city)

	content, err := ai.makeAPIRequest(systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("failed to get top attractions: %v", err)
	}

	// Parse the AI response - remove markdown code blocks if present
	jsonContent := RemoveMarkdownCodeBlocks(content)

	// Parse the JSON response
	var attractionResp types.AttractionResponse
	if err := json.Unmarshal([]byte(jsonContent), &attractionResp); err != nil {
		return fmt.Errorf("failed to parse attractions response: %v", err)
	}

	places := FlattenAttractionResponse(attractionResp, areaKey, city)

	fmt.Printf("Successfully parsed %d places for %s\n", len(places), city)

	// Fix places exact locations using Overpass with 2km bounding box
	if ai.overpass != nil {
		for _, place := range places {
			// Generate 2km bounding box around approximate coordinates and search for exact location
			place.Lat, place.Lon = ai.overpass.FixLocation(place.Name, city, place.Lat, place.Lon)
		}
	}

	// Save places to database
	if ai.supabase != nil {
		if err := ai.supabase.SavePlaces(places, areaKey); err != nil {
			return fmt.Errorf("failed to save places to database: %v", err)
		}
		fmt.Printf("Successfully saved %d places to database for area %s\n", len(places), areaKey)
	}

	return nil
}
