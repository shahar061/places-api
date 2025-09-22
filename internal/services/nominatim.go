package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"places_api/internal/types"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NominatimService handles geocoding requests to OpenStreetMap Nominatim
type NominatimService struct {
	baseURL     string
	userAgent   string
	httpClient  *http.Client
	rateLimiter *rateLimiter
}

// rateLimiter ensures compliance with Nominatim's 1 request per second limit
type rateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
}

// NominatimResponse represents the response from Nominatim API
type NominatimResponse struct {
	PlaceID     int64                  `json:"place_id"`
	License     string                 `json:"licence"`
	OSMType     string                 `json:"osm_type"`
	OSMId       int64                  `json:"osm_id"`
	Boundingbox []string               `json:"boundingbox"`
	Lat         string                 `json:"lat"`
	Lon         string                 `json:"lon"`
	DisplayName string                 `json:"display_name"`
	Type        string                 `json:"type"`
	Importance  float64                `json:"importance"`
	Address     map[string]interface{} `json:"address"`
}

// NewNominatimService creates a new Nominatim service
func NewNominatimService() *NominatimService {
	return &NominatimService{
		baseURL:   "https://nominatim.openstreetmap.org",
		userAgent: "PlacesAPI/1.0 (https://github.com/yourorg/places_api)", // UPDATE THIS
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		rateLimiter: &rateLimiter{},
	}
}

// wait enforces rate limiting - max 1 request per second
func (rl *rateLimiter) wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	timeSinceLastCall := time.Since(rl.lastCall)
	if timeSinceLastCall < time.Second {
		time.Sleep(time.Second - timeSinceLastCall)
	}
	rl.lastCall = time.Now()
}

// GeocodeStructured performs structured geocoding using city, region, country
func (ns *NominatimService) GeocodeStructured(queryLoc types.QueryLocation) (*types.Area, error) {
	// Rate limiting compliance
	ns.rateLimiter.wait()

	// Build structured query parameters
	params := url.Values{}
	params.Set("format", "json")
	params.Set("addressdetails", "1")
	params.Set("limit", "1")
	params.Set("accept-language", "en")

	// Use structured search for better accuracy
	if queryLoc.City != "" {
		params.Set("city", queryLoc.City)
	}
	if queryLoc.Region != "" {
		params.Set("state", queryLoc.Region)
	}
	if queryLoc.Country != "" {
		params.Set("country", queryLoc.Country)
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s/search?%s", ns.baseURL, params.Encode())

	// Create request with proper User-Agent
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", ns.userAgent)

	// Make request
	resp, err := ns.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status %d", resp.StatusCode)
	}

	// Parse response
	var nominatimResults []NominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&nominatimResults); err != nil {
		return nil, fmt.Errorf("failed to parse nominatim response: %v", err)
	}

	if len(nominatimResults) == 0 {
		return nil, fmt.Errorf("no results found for query")
	}

	// Convert first result to Area
	result := nominatimResults[0]
	area, err := ns.convertToArea(result, queryLoc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert nominatim result: %v", err)
	}

	return area, nil
}

// convertToArea converts Nominatim response to Area type
func (ns *NominatimService) convertToArea(result NominatimResponse, queryLoc types.QueryLocation) (*types.Area, error) {
	// Parse coordinates
	lat, err := strconv.ParseFloat(result.Lat, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid latitude: %v", err)
	}
	lon, err := strconv.ParseFloat(result.Lon, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid longitude: %v", err)
	}

	// Parse bounding box
	var bbox types.BoundingBox
	if len(result.Boundingbox) == 4 {
		if south, err := strconv.ParseFloat(result.Boundingbox[0], 64); err == nil {
			bbox.SouthLat = south
		}
		if north, err := strconv.ParseFloat(result.Boundingbox[1], 64); err == nil {
			bbox.NorthLat = north
		}
		if west, err := strconv.ParseFloat(result.Boundingbox[2], 64); err == nil {
			bbox.WestLon = west
		}
		if east, err := strconv.ParseFloat(result.Boundingbox[3], 64); err == nil {
			bbox.EastLon = east
		}
	}

	// Generate area key
	areaKey := ns.generateAreaKey(queryLoc)

	// Extract country code from address
	countryCode := ""
	if addr, ok := result.Address["country_code"].(string); ok {
		countryCode = strings.ToUpper(addr)
	}

	// Determine type and admin level based on OSM type
	areaType := "locality"
	adminLevel := 8
	switch result.Type {
	case "city", "town":
		areaType = "city"
		adminLevel = 8
	case "village", "hamlet":
		areaType = "locality"
		adminLevel = 10
	case "state", "province":
		areaType = "region"
		adminLevel = 4
	case "country":
		areaType = "country"
		adminLevel = 2
	}

	area := &types.Area{
		AreaKey:       areaKey,
		Name:          ns.extractName(result, queryLoc),
		Type:          areaType,
		CountryCode:   countryCode,
		AdminLevel:    adminLevel,
		Center:        types.Coordinate{Lat: lat, Lon: lon},
		BBox:          bbox,
		RefreshedAt:   time.Now(),
		RefreshQueued: false,
	}

	return area, nil
}

// generateAreaKey creates a unique area key from the query location
func (ns *NominatimService) generateAreaKey(queryLoc types.QueryLocation) string {
	parts := []string{}
	if queryLoc.City != "" {
		parts = append(parts, strings.ToLower(queryLoc.City))
	}
	if queryLoc.Region != "" {
		parts = append(parts, strings.ToLower(queryLoc.Region))
	}
	if queryLoc.Country != "" {
		parts = append(parts, strings.ToLower(queryLoc.Country))
	}
	return strings.Join(parts, "_")
}

// extractName gets the best display name for the area
func (ns *NominatimService) extractName(result NominatimResponse, queryLoc types.QueryLocation) string {
	if queryLoc.City != "" {
		return queryLoc.City
	}
	if queryLoc.Region != "" {
		return queryLoc.Region
	}
	if queryLoc.Country != "" {
		return queryLoc.Country
	}

	// Fallback to display name from Nominatim
	parts := strings.Split(result.DisplayName, ",")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}

	return result.DisplayName
}
