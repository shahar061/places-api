package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"places_api/internal/config"
	"places_api/internal/types"
	"strings"
	"time"

	"github.com/supabase-community/postgrest-go"
)

const (
	areasTable  = "areas"
	placesTable = "places"
)

// SupabaseService handles all interactions with Supabase
type SupabaseService struct {
	client     *postgrest.Client
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewSupabaseService creates a new Supabase service instance
func NewSupabaseService(cfg *config.DatabaseConfig) (*SupabaseService, error) {
	if cfg.SupabaseURL == "" || cfg.SupabaseKey == "" {
		return nil, fmt.Errorf("supabase URL and key are required")
	}

	// Debug: Print the configuration values (mask the key for security)
	fmt.Printf("Supabase URL: %s\n", cfg.SupabaseURL)
	fmt.Printf("Supabase Key: %s...%s\n", cfg.SupabaseKey[:8], cfg.SupabaseKey[len(cfg.SupabaseKey)-8:])

	// Ensure URL ends with /rest/v1
	supabaseURL := cfg.SupabaseURL

	fmt.Printf("Final URL: %s\n", supabaseURL)

	client := postgrest.NewClient(supabaseURL, "", map[string]string{
		"apikey":        cfg.SupabaseKey,
		"Authorization": "Bearer " + cfg.SupabaseKey,
	})

	return &SupabaseService{
		client:     client,
		baseURL:    supabaseURL,
		apiKey:     cfg.SupabaseKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// ResolveArea resolves a text query to canonical area(s)
func (s *SupabaseService) ResolveArea(query string, multi bool) (interface{}, error) {
	// Query the areas table
	response, _, err := s.client.From(areasTable).Select("*", "", false).Eq("area_key", query).Execute()
	if err != nil {
		fmt.Printf("Supabase query error: %v\n", err)
		return nil, fmt.Errorf("failed to query areas table: %v", err)
	}

	// Parse the JSON response into AreaFlat structs
	var flatAreas []types.AreaFlat
	if err := json.Unmarshal(response, &flatAreas); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// Handle empty results
	if len(flatAreas) == 0 {
		if multi {
			return []types.Area{}, nil
		}
		return types.Area{}, nil
	}

	// Convert flat areas to nested Area structs
	if multi {
		areas := make([]types.Area, len(flatAreas))
		for i, flatArea := range flatAreas {
			areas[i] = *types.FromFlat(&flatArea)
		}
		return areas, nil
	} else {
		// Return single area (first result)
		area := types.FromFlat(&flatAreas[0])
		return *area, nil
	}
}

// GetAreaChildren retrieves child areas for hierarchical navigation
func (s *SupabaseService) GetAreaChildren(parent string, categoryTypes string, limit, offset int) (map[string]interface{}, error) {
	// TODO: Implement child area lookup
	// This would query areas table with parent relationships
	return map[string]interface{}{
		"parent":   parent,
		"children": []interface{}{},
	}, nil
}

// GetTopPlaces retrieves ranked places for an area from cache
func (s *SupabaseService) GetTopPlaces(areaKey string, categories string, limit, offset int) ([]types.Place, error) {
	if s.httpClient == nil {
		return nil, fmt.Errorf("supabase service not initialized")
	}

	fmt.Printf("Getting top places for area: %s, categories: %s, limit: %d, offset: %d\n", areaKey, categories, limit, offset)

	// Build query URL - search for places where area_key equals areaKey or id starts with areaKey
	url := fmt.Sprintf("%s/places?select=*", s.baseURL)

	// Filter by area_key or id starting with area key
	url += fmt.Sprintf("&or=(area_key.eq.%s,id.like.%s%%2A)", areaKey, areaKey)

	// Filter by categories if provided
	if categories != "" {
		categoryList := strings.Split(categories, ",")
		categoryFilters := make([]string, len(categoryList))
		for i, cat := range categoryList {
			categoryFilters[i] = fmt.Sprintf("category.eq.%s", strings.TrimSpace(cat))
		}
		url += fmt.Sprintf("&or=(%s)", strings.Join(categoryFilters, ","))
	}

	// Order by popularity descending
	url += "&order=popularity.desc"

	// Apply limit and offset
	if limit > 0 {
		url += fmt.Sprintf("&limit=%d", limit)
	}
	if offset > 0 {
		url += fmt.Sprintf("&offset=%d", offset)
	}

	fmt.Printf("Querying: %s\n", url)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("apikey", s.apiKey)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("supabase returned status %d", resp.StatusCode)
	}

	// Parse response
	var placesData []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&placesData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// Convert to Place structs
	places := make([]types.Place, 0, len(placesData))
	for _, data := range placesData {
		place := types.Place{}

		if id, ok := data["id"].(string); ok {
			place.ID = id
		}
		if name, ok := data["name"].(string); ok {
			place.Name = name
		}
		if category, ok := data["category"].(string); ok {
			place.Category = category
		}
		if lat, ok := data["lat"].(float64); ok {
			place.Lat = lat
		}
		if lon, ok := data["lon"].(float64); ok {
			place.Lon = lon
		}
		if address, ok := data["address"].(string); ok {
			place.Address = address
		}
		if popularity, ok := data["popularity"].(float64); ok {
			place.Popularity = popularity
		}
		if description, ok := data["description"].(string); ok {
			place.Description = description
		}

		// Parse updated_at timestamp
		if updatedAtStr, ok := data["updated_at"].(string); ok && updatedAtStr != "" {
			if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
				place.UpdatedAt = updatedAt
			}
		}
		if place.UpdatedAt.IsZero() {
			place.UpdatedAt = time.Now()
		}

		places = append(places, place)
	}

	fmt.Printf("✓ Retrieved %d places for area %s\n", len(places), areaKey)
	return places, nil
}

// GetNearbyPlaces performs radial search using PostGIS
func (s *SupabaseService) GetNearbyPlaces(lat, lon float64, radius, limit int, categories string) ([]types.Place, error) {
	// TODO: Implement PostGIS radial search
	// This would use ST_DWithin or similar PostGIS functions
	return []types.Place{}, nil
}

// SearchPlaces performs typeahead search over cached place names
func (s *SupabaseService) SearchPlaces(areaKey, query, categories string, limit int) ([]types.Place, error) {
	// TODO: Implement fuzzy/prefix search over place names
	// This would use PostgreSQL full-text search or trigram matching
	return []types.Place{}, nil
}

// GetPlaceDetails retrieves detailed information for a specific place
func (s *SupabaseService) GetPlaceDetails(id string) (*types.PlaceDetail, error) {
	// TODO: Implement place detail lookup
	// This would query the places table with detailed information
	return nil, fmt.Errorf("place not found")
}

// GetAreaStatus retrieves cache status and statistics for an area
func (s *SupabaseService) GetAreaStatus(areaKey string) (*types.AreaStatus, error) {
	// TODO: Implement area status lookup
	// This would query area cache status and job information
	return nil, nil
}

// SaveArea saves an area to the database
func (s *SupabaseService) SaveArea(area *types.Area) error {
	// Convert to flat structure for database
	flatArea := area.ToFlat()

	// Insert into database (upsert to handle duplicates)
	_, _, err := s.client.From(areasTable).Upsert(flatArea, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to save area to database: %v", err)
	}

	return nil
}

// SavePlaces saves places to the database with upsert functionality
func (s *SupabaseService) SavePlaces(places []types.Place, areaKey string) error {
	if len(places) == 0 {
		return nil
	}

	// Convert places to database format and add area_key
	placesData := make([]map[string]interface{}, len(places))
	for i, place := range places {
		placesData[i] = map[string]interface{}{
			"id":          place.ID,
			"name":        place.Name,
			"category":    place.Category,
			"lat":         place.Lat,
			"lon":         place.Lon,
			"address":     place.Address,
			"area_key":    areaKey,
			"popularity":  place.Popularity,
			"distance_m":  nil, // Distance is only relevant for nearby searches
			"description": place.Description,
			"updated_at":  time.Now(),
		}
	}

	// Upsert places to database
	_, _, err := s.client.From(placesTable).Upsert(placesData, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to save places to database: %v", err)
	}

	return nil
}

// BootstrapArea queues background refresh for an area
func (s *SupabaseService) BootstrapArea(areaKey string, categories []string) (string, error) {
	// TODO: Implement job queueing for area bootstrap
	// This would insert into a jobs table or queue system
	return "", nil
}
