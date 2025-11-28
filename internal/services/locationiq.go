package services

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"places_api/internal/types"
	"strings"
	"time"
)

// LocationIQResponse represents the response from LocationIQ API
type LocationIQResponse struct {
	PlaceID     string `json:"place_id"`
	Licence     string `json:"licence"`
	OsmType     string `json:"osm_type"`
	OsmID       int64  `json:"osm_id"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
	Address     struct {
		Name          string `json:"name"`
		Road          string `json:"road"`
		HouseNumber   string `json:"house_number"`
		Neighbourhood string `json:"neighbourhood"`
		Suburb        string `json:"suburb"`
		City          string `json:"city"`
		County        string `json:"county"`
		State         string `json:"state"`
		Postcode      string `json:"postcode"`
		Country       string `json:"country"`
		CountryCode   string `json:"country_code"`
	} `json:"address"`
	BoundingBox []string `json:"boundingbox"`
}

// LocationIQService handles geocoding using LocationIQ API
type LocationIQService struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewLocationIQService creates a new LocationIQ service instance
func NewLocationIQService(apiKey string) *LocationIQService {
	if apiKey == "" {
		fmt.Println("⚠️  LocationIQ API key not configured - LocationIQ fallback will be disabled")
		return nil
	}

	return &LocationIQService{
		apiKey:  apiKey,
		baseURL: "https://us1.locationiq.com/v1",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SearchLocation searches for a location by name using LocationIQ
// Returns the first matching result or error if not found
func (s *LocationIQService) SearchLocation(locationName string) (*LocationIQResponse, error) {
	if s == nil {
		return nil, fmt.Errorf("LocationIQ service not initialized (API key missing)")
	}

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("key", s.apiKey)
	queryParams.Set("q", locationName)
	queryParams.Set("format", "json")
	queryParams.Set("addressdetails", "1")
	queryParams.Set("limit", "1")

	// Build the URL
	requestURL := fmt.Sprintf("%s/search?%s", s.baseURL, queryParams.Encode())

	// Make the request
	resp, err := s.httpClient.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query LocationIQ: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LocationIQ API returned status %d", resp.StatusCode)
	}

	// Parse response
	var results []LocationIQResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to parse LocationIQ response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found for query: %s", locationName)
	}

	return &results[0], nil
}

// IsExploratoryActivity checks if an activity suggests area exploration vs specific point
func (s *LocationIQService) IsExploratoryActivity(activityName string) bool {
	exploratoryKeywords := []string{
		"explore", "wander", "discover", "browse",
		"stroll", "walk around", "experience",
		"local", "authentic", "traditional",
		"eateries", "shops", "markets",
	}

	nameLower := strings.ToLower(activityName)
	for _, keyword := range exploratoryKeywords {
		if strings.Contains(nameLower, keyword) {
			return true
		}
	}

	return false
}

// IsAreaLocation checks if the OSM location type represents an area/region
func (s *LocationIQService) IsAreaLocation(location *LocationIQResponse) bool {
	// Relations (R) are typically areas
	if location.OsmType != "relation" {
		return false
	}

	// Check based on place type in address
	// LocationIQ doesn't have a direct "type" field like Photon, so we infer from the presence of city-level data
	return location.Address.City != "" && location.Address.Road == ""
}

// EnrichActivity enriches an activity with location data from LocationIQ
// Returns true if enrichment was successful, false otherwise
func (s *LocationIQService) EnrichActivity(activity *types.ItineraryActivity) bool {
	return s.EnrichActivityWithContext(activity, nil)
}

// EnrichActivityWithContext enriches an activity with location data using destination context
func (s *LocationIQService) EnrichActivityWithContext(activity *types.ItineraryActivity, destinationContext *types.TripDestination) bool {
	if s == nil {
		return false
	}

	if activity.LocationName == "" {
		return false
	}

	// Build query with destination context
	searchQuery := activity.LocationName
	if destinationContext != nil {
		searchQuery = s.BuildContextualQuery(activity.LocationName, destinationContext)
	}

	// Search for location with context
	location, err := s.SearchLocation(searchQuery)
	if err != nil {
		// Fallback: try without context
		if destinationContext != nil {
			location, err = s.SearchLocation(activity.LocationName)
			if err != nil {
				return false
			}
		} else {
			return false
		}
	}

	// Validate result is within reasonable distance of destination
	if destinationContext != nil && destinationContext.Latitude != nil && destinationContext.Longitude != nil {
		if !s.ValidateLocationDistance(location, *destinationContext.Latitude, *destinationContext.Longitude) {
			return false
		}
	}

	// Parse center coordinates
	var lat, lon float64
	if _, err := fmt.Sscanf(location.Lat, "%f", &lat); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(location.Lon, "%f", &lon); err != nil {
		return false
	}

	// Determine if this should be an area or point activity
	isExploratory := s.IsExploratoryActivity(activity.Name)
	isArea := s.IsAreaLocation(location)
	hasBbox := len(location.BoundingBox) == 4

	// If exploratory activity and we have bbox data, use area mode
	// LocationIQ bbox format: [minLat, maxLat, minLon, maxLon]
	if (isExploratory || isArea) && hasBbox {
		var minLat, maxLat, minLon, maxLon float64
		fmt.Sscanf(location.BoundingBox[0], "%f", &minLat)
		fmt.Sscanf(location.BoundingBox[1], "%f", &maxLat)
		fmt.Sscanf(location.BoundingBox[2], "%f", &minLon)
		fmt.Sscanf(location.BoundingBox[3], "%f", &maxLon)

		activity.BboxMinLon = &minLon
		activity.BboxMinLat = &minLat
		activity.BboxMaxLon = &maxLon
		activity.BboxMaxLat = &maxLat
		activity.ActivityMode = "area"

		// Still set center point as fallback
		activity.Latitude = &lat
		activity.Longitude = &lon
	} else {
		// Use point mode
		activity.Latitude = &lat
		activity.Longitude = &lon
		activity.ActivityMode = "point"
	}

	// Set address
	if location.DisplayName != "" {
		activity.Address = location.DisplayName
	}

	return true
}

// FormatAddress creates a human-readable address from LocationIQ response
func (s *LocationIQService) FormatAddress(location *LocationIQResponse) string {
	// LocationIQ provides a nice display_name, but we can also build from address parts
	if location.DisplayName != "" {
		return location.DisplayName
	}

	var parts []string

	addr := location.Address

	// Add name if available
	if addr.Name != "" {
		parts = append(parts, addr.Name)
	}

	// Add street and house number
	if addr.Road != "" {
		road := addr.Road
		if addr.HouseNumber != "" {
			road = addr.HouseNumber + " " + road
		}
		parts = append(parts, road)
	}

	// Add postcode and city
	if addr.Postcode != "" {
		parts = append(parts, addr.Postcode)
	}
	if addr.City != "" {
		parts = append(parts, addr.City)
	}

	// Add state
	if addr.State != "" {
		parts = append(parts, addr.State)
	}

	// Add country
	if addr.Country != "" {
		parts = append(parts, addr.Country)
	}

	if len(parts) == 0 {
		return ""
	}

	return joinNonEmpty(parts, ", ")
}

// Helper function to join non-empty strings
func joinNonEmpty(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if part == "" {
			continue
		}
		if result != "" {
			result += sep
		}
		result += part
		_ = i
	}
	return result
}

// BuildContextualQuery creates a search query with geographical context
func (s *LocationIQService) BuildContextualQuery(locationName string, destination *types.TripDestination) string {
	if destination == nil {
		return locationName
	}

	// Start with the location name
	query := locationName

	// Add city context
	if destination.DisplayName != "" {
		query += ", " + destination.DisplayName
	}

	// Add country context if available
	if destination.Country != nil && *destination.Country != "" {
		query += ", " + *destination.Country
	} else if destination.Location != nil && destination.Location.Country != "" {
		query += ", " + destination.Location.Country
	}

	return query
}

// ValidateLocationDistance checks if a geocoding result is within reasonable distance of destination
func (s *LocationIQService) ValidateLocationDistance(location *LocationIQResponse, destLat, destLon float64) bool {
	if location == nil {
		return false
	}

	// Parse result coordinates
	var resultLat, resultLon float64
	if _, err := fmt.Sscanf(location.Lat, "%f", &resultLat); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(location.Lon, "%f", &resultLon); err != nil {
		return false
	}

	// Calculate distance
	distance := s.HaversineDistance(destLat, destLon, resultLat, resultLon)

	// Threshold: 200km (same as Photon)
	maxDistanceKm := 200.0

	return distance <= maxDistanceKm
}

// HaversineDistance calculates the great-circle distance between two points (in kilometers)
func (s *LocationIQService) HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	// Convert to radians
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Haversine formula
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}
