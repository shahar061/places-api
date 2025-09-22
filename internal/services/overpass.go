package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"places_api/internal/types"
	"strings"
	"sync"
	"time"
)

// OverpassService handles location data requests to Overpass API (OpenStreetMap)
type OverpassService struct {
	baseURL      string
	alternateURL string
	userAgent    string
	httpClient   *http.Client
	rateLimiter  *overpassRateLimiter
}

// overpassRateLimiter ensures compliance with Overpass API best practices
// Recommended: max 2 requests per second to avoid server overload
type overpassRateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
}

// OverpassResponse represents the response from Overpass API
type OverpassResponse struct {
	Version   float64           `json:"version"`
	Generator string            `json:"generator"`
	Elements  []OverpassElement `json:"elements"`
}

// OverpassElement represents a single element (node, way, relation) from Overpass
type OverpassElement struct {
	Type   string            `json:"type"`
	ID     int64             `json:"id"`
	Lat    *float64          `json:"lat,omitempty"`    // For nodes
	Lon    *float64          `json:"lon,omitempty"`    // For nodes
	Center *OverpassCenter   `json:"center,omitempty"` // For ways/relations with out center;
	Tags   map[string]string `json:"tags,omitempty"`
}

// OverpassCenter represents the center point of a way or relation
type OverpassCenter struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// NewOverpassService creates a new Overpass service
func NewOverpassService() *OverpassService {
	return &OverpassService{
		baseURL:      "https://overpass-api.de/api",
		alternateURL: "https://overpass.kumi.systems/api",                      // Alternative server
		userAgent:    "PlacesAPI/1.0 (+https://github.com/yourorg/places_api)", // UPDATE THIS
		httpClient: &http.Client{
			Timeout: 15 * time.Second, // Shorter timeout to avoid server drops
		},
		rateLimiter: &overpassRateLimiter{},
	}
}

// TestConnection tests basic connectivity to Overpass API
func (os *OverpassService) TestConnection() error {
	// Very simple test query - just get the API status
	query := `[out:json][timeout:5];
node(0);
out;`

	fmt.Println("Testing Overpass API connection...")

	response, err := os.executeQueryWithRetry(query, 2)
	if err != nil {
		return fmt.Errorf("overpass API connection test failed: %v", err)
	}

	fmt.Printf("✓ Overpass API connection successful, got %d elements\n", len(response.Elements))
	return nil
}

// wait enforces rate limiting - max 1 request per 2 seconds for Overpass API
// Very conservative to avoid server overload and timeouts
func (rl *overpassRateLimiter) wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	timeSinceLastCall := time.Since(rl.lastCall)
	minInterval := 2 * time.Second // 1 request per 2 seconds - very conservative
	if timeSinceLastCall < minInterval {
		time.Sleep(minInterval - timeSinceLastCall)
	}
	rl.lastCall = time.Now()
}

// FindPlaceLocation searches for a place by name in a specific city/area
func (os *OverpassService) FindPlaceLocation(placeName, cityName string) (*types.Place, error) {
	// Rate limiting compliance
	os.rateLimiter.wait()

	// Try multiple query strategies if the first fails
	queries := []string{
		os.buildOverpassQuery(placeName, cityName),
		os.buildSimpleQuery(placeName, cityName),
		os.buildFallbackQuery(placeName),
	}

	var lastErr error
	for i, query := range queries {
		fmt.Printf("Trying Overpass query %d for %s in %s\n", i+1, placeName, cityName)

		// Try the query with retry for timeout errors
		response, err := os.executeQueryWithRetry(query, 2)
		if err != nil {
			lastErr = err
			fmt.Printf("Query %d failed: %v\n", i+1, err)
			continue
		}

		// Parse and find the best match
		place, err := os.parseResponse(response, placeName, cityName)
		if err != nil {
			lastErr = err
			fmt.Printf("Parsing failed for query %d: %v\n", i+1, err)
			continue
		}

		return place, nil
	}

	return nil, fmt.Errorf("all overpass queries failed, last error: %v", lastErr)
}

// buildOverpassQuery constructs an Overpass QL query to find a place
// Now takes bounding box coordinates instead of relying on geocodeArea
func (os *OverpassService) buildOverpassQueryWithBBox(placeName string, south, west, north, east float64) string {
	// Escape quotes and clean the place name
	cleanPlaceName := strings.ReplaceAll(placeName, `"`, `\"`)

	// Also get normalized version for better matching
	normalizedName := os.normalizeName(placeName)
	cleanNormalizedName := strings.ReplaceAll(normalizedName, `"`, `\"`)

	// Optimize query for speed and reliability:
	// 1. Shorter timeout (10s works better than 25s)
	// 2. Exact name match (faster than regex)
	// 3. Smaller bbox (reduce area by 20% for faster processing)
	// 4. Search both original and normalized names
	centerLat := (south + north) / 2
	centerLon := (west + east) / 2
	latRange := (north - south) * 0.8 // Reduce by 20%
	lonRange := (east - west) * 0.8

	newSouth := centerLat - latRange/2
	newNorth := centerLat + latRange/2
	newWest := centerLon - lonRange/2
	newEast := centerLon + lonRange/2

	var query string
	if cleanNormalizedName != cleanPlaceName {
		// Search both original and normalized names
		query = fmt.Sprintf(`[out:json][timeout:10];
(
  node[name="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  node[name="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  node["name:en"="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  node["name:en"="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  way[name="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  way[name="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  way["name:en"="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  way["name:en"="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
);
out center;`,
			cleanPlaceName, newSouth, newWest, newNorth, newEast,
			cleanNormalizedName, newSouth, newWest, newNorth, newEast,
			cleanPlaceName, newSouth, newWest, newNorth, newEast,
			cleanNormalizedName, newSouth, newWest, newNorth, newEast,
			cleanPlaceName, newSouth, newWest, newNorth, newEast,
			cleanNormalizedName, newSouth, newWest, newNorth, newEast,
			cleanPlaceName, newSouth, newWest, newNorth, newEast,
			cleanNormalizedName, newSouth, newWest, newNorth, newEast)
	} else {
		// Original and normalized are the same, use simpler query
		query = fmt.Sprintf(`[out:json][timeout:10];
(
  node[name="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  node["name:en"="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  way[name="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
  way["name:en"="%s"](bbox:%.8f,%.8f,%.8f,%.8f);
);
out center;`,
			cleanPlaceName, newSouth, newWest, newNorth, newEast,
			cleanPlaceName, newSouth, newWest, newNorth, newEast,
			cleanPlaceName, newSouth, newWest, newNorth, newEast,
			cleanPlaceName, newSouth, newWest, newNorth, newEast)
	}

	return query
}

// buildOverpassQuery constructs an Overpass QL query to find a place (fallback version)
func (os *OverpassService) buildOverpassQuery(placeName, cityName string) string {
	// Escape quotes and clean the place name
	cleanPlaceName := strings.ReplaceAll(placeName, `"`, `\"`)

	// Fallback to simple global search if no bounding box available
	query := fmt.Sprintf(`[out:json][timeout:15];
(
  node[name~"%s",i];
  way[name~"%s",i];
  relation[name~"%s",i];
);
out center;`,
		cleanPlaceName, cleanPlaceName, cleanPlaceName)

	return query
}

// buildSimpleQuery constructs a simpler query without geocodeArea
func (os *OverpassService) buildSimpleQuery(placeName, cityName string) string {
	cleanPlaceName := strings.ReplaceAll(placeName, `"`, `\"`)

	// Also get normalized version for better matching
	normalizedName := os.normalizeName(placeName)
	cleanNormalizedName := strings.ReplaceAll(normalizedName, `"`, `\"`)

	var query string
	if cleanNormalizedName != cleanPlaceName {
		// Search both original and normalized names plus English names
		query = fmt.Sprintf(`[out:json][timeout:8];
(
  node[name="%s"];
  node[name="%s"];
  node["name:en"="%s"];
  node["name:en"="%s"];
  way[name="%s"];
  way[name="%s"];
  way["name:en"="%s"];
  way["name:en"="%s"];
);
out center;`,
			cleanPlaceName, cleanNormalizedName, cleanPlaceName, cleanNormalizedName,
			cleanPlaceName, cleanNormalizedName, cleanPlaceName, cleanNormalizedName)
	} else {
		// Original and normalized are the same, use simpler query
		query = fmt.Sprintf(`[out:json][timeout:8];
(
  node[name="%s"];
  node["name:en"="%s"];
  way[name="%s"];
  way["name:en"="%s"];
);
out center;`,
			cleanPlaceName, cleanPlaceName, cleanPlaceName, cleanPlaceName)
	}

	return query
}

// buildFallbackQuery constructs the most basic query possible
func (os *OverpassService) buildFallbackQuery(placeName string) string {
	cleanPlaceName := strings.ReplaceAll(placeName, `"`, `\"`)

	// Most basic query - just nodes with the name
	query := fmt.Sprintf(`[out:json][timeout:5];
node[name="%s"];
out;`, cleanPlaceName)

	return query
}

// executeQuery sends the Overpass QL query to the API using the default server
func (os *OverpassService) executeQuery(query string) (*OverpassResponse, error) {
	return os.executeQueryWithServer(query, os.baseURL)
}

// executeQueryWithServer sends the Overpass QL query to a specific server
func (os *OverpassService) executeQueryWithServer(query string, serverURL string) (*OverpassResponse, error) {
	// Debug: log the query being sent
	fmt.Printf("Overpass Query to %s: %s\n", serverURL, query)

	// Prepare form data
	data := url.Values{}
	data.Set("data", query)

	// Create request
	req, err := http.NewRequest("POST", serverURL+"/interpreter", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", os.userAgent)

	// Make request
	resp, err := os.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body for better error handling
	var respBody bytes.Buffer
	respBody.ReadFrom(resp.Body)
	bodyContent := respBody.String()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass API returned status %d: %s", resp.StatusCode, bodyContent)
	}

	// Parse JSON response
	var overpassResp OverpassResponse
	if err := json.Unmarshal([]byte(bodyContent), &overpassResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v (body: %s)", err, bodyContent)
	}

	return &overpassResp, nil
}

// executeQueryWithRetry executes a query with retry logic for timeout errors
func (os *OverpassService) executeQueryWithRetry(query string, maxRetries int) (*OverpassResponse, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("Retrying query (attempt %d/%d)...\n", attempt, maxRetries)
			// Wait a bit longer before retry
			time.Sleep(3 * time.Second)
		}

		// Try alternate server on second attempt for timeout errors
		var response *OverpassResponse
		var err error

		if attempt == 2 {
			fmt.Printf("Trying alternate Overpass server...\n")
			response, err = os.executeQueryWithServer(query, os.alternateURL)
		} else {
			response, err = os.executeQuery(query)
		}

		if err != nil {
			lastErr = err
			fmt.Printf("Query attempt %d failed: %v\n", attempt, err)

			// Check if it's a timeout error
			if os.isTimeoutError(err) {
				fmt.Printf("Timeout on attempt %d, will retry...\n", attempt)
				continue
			}

			// For non-timeout errors, don't retry immediately but still log
			fmt.Printf("Non-timeout error on attempt %d: %v\n", attempt, err)
			return nil, err
		}

		return response, nil
	}

	return nil, fmt.Errorf("query failed after %d attempts, last error: %v", maxRetries, lastErr)
}

// isTimeoutError checks if an error is a timeout error
func (os *OverpassService) isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context cancellation")
}

// parseResponse extracts the best matching place from Overpass response (basic version)
func (os *OverpassService) parseResponse(response *OverpassResponse, placeName, cityName string) (*types.Place, error) {
	if len(response.Elements) == 0 {
		return nil, fmt.Errorf("no places found matching '%s'", placeName)
	}

	// Find the best match - prioritize exact name matches and nodes over ways
	var bestMatch *OverpassElement
	var bestScore int

	for _, element := range response.Elements {
		if element.Tags == nil {
			continue
		}

		name, hasName := element.Tags["name"]
		if !hasName {
			continue
		}

		// Calculate match score
		score := os.calculateMatchScore(name, placeName, &element)

		if bestMatch == nil || score > bestScore {
			elementCopy := element
			bestMatch = &elementCopy
			bestScore = score
		}
	}

	if bestMatch == nil {
		return nil, fmt.Errorf("no suitable match found for '%s' (checked %d elements)", placeName, len(response.Elements))
	}

	// Convert to Place
	place := os.convertToPlace(bestMatch, cityName)
	return place, nil
}

// calculateMatchScore rates how well an element matches our search
func (os *OverpassService) calculateMatchScore(elementName, searchName string, element *OverpassElement) int {
	score := 0

	// Check both main name and English name
	var nameToCheck string
	var englishName string

	if element.Tags != nil {
		nameToCheck = element.Tags["name"]
		englishName = element.Tags["name:en"]
	}

	// Normalize both search name and element names for better matching
	normalizedSearch := os.normalizeName(searchName)
	normalizedMain := os.normalizeName(nameToCheck)
	normalizedEnglish := os.normalizeName(englishName)

	// Exact name match gets highest score (check both main name and English name)
	if strings.EqualFold(normalizedMain, normalizedSearch) || strings.EqualFold(normalizedEnglish, normalizedSearch) {
		score += 100
	} else if strings.EqualFold(nameToCheck, searchName) || strings.EqualFold(englishName, searchName) {
		score += 95 // Original exact match (slightly lower than normalized)
	} else if strings.Contains(strings.ToLower(normalizedMain), strings.ToLower(normalizedSearch)) ||
		strings.Contains(strings.ToLower(normalizedEnglish), strings.ToLower(normalizedSearch)) {
		score += 60 // Normalized partial match
	} else if strings.Contains(strings.ToLower(nameToCheck), strings.ToLower(searchName)) ||
		strings.Contains(strings.ToLower(englishName), strings.ToLower(searchName)) {
		score += 50 // Original partial match
	} else if strings.Contains(strings.ToLower(normalizedSearch), strings.ToLower(normalizedMain)) ||
		strings.Contains(strings.ToLower(normalizedSearch), strings.ToLower(normalizedEnglish)) {
		score += 35 // Normalized reverse partial match
	} else if strings.Contains(strings.ToLower(searchName), strings.ToLower(nameToCheck)) ||
		strings.Contains(strings.ToLower(searchName), strings.ToLower(englishName)) {
		score += 30 // Original reverse partial match
	}

	// Bonus for having coordinates (check both direct coordinates and center)
	if (element.Lat != nil && element.Lon != nil) || element.Center != nil {
		score += 20
	}

	// Bonus for having relevant tags
	if element.Tags != nil {
		if _, hasAmenity := element.Tags["amenity"]; hasAmenity {
			score += 10
		}
		if _, hasTourism := element.Tags["tourism"]; hasTourism {
			score += 10
		}
		if _, hasShop := element.Tags["shop"]; hasShop {
			score += 10
		}
		if _, hasHistoric := element.Tags["historic"]; hasHistoric {
			score += 10 // Historic places are often landmarks
		}
	}

	// Preference for nodes over ways (usually more precise)
	if element.Type == "node" {
		score += 5
	}

	return score
}

// convertToPlace converts an OverpassElement to a Place
func (os *OverpassService) convertToPlace(element *OverpassElement, cityName string) *types.Place {
	place := &types.Place{
		ID:        fmt.Sprintf("osm_%s_%d", element.Type, element.ID),
		Name:      element.Tags["name"],
		UpdatedAt: time.Now(),
	}

	// Set coordinates if available (check both direct coordinates and center)
	if element.Lat != nil && element.Lon != nil {
		// Node coordinates
		place.Lat = *element.Lat
		place.Lon = *element.Lon
	} else if element.Center != nil {
		// Way/relation center coordinates
		place.Lat = element.Center.Lat
		place.Lon = element.Center.Lon
	}

	// Determine category from OSM tags
	place.Category = os.inferCategory(element.Tags)

	// Build address
	place.Address = os.buildAddress(element.Tags, cityName)

	// Set popularity based on tags (rough heuristic)
	place.Popularity = os.inferPopularity(element.Tags)

	return place
}

// inferCategory maps OSM tags to our category system
func (os *OverpassService) inferCategory(tags map[string]string) string {
	if amenity, ok := tags["amenity"]; ok {
		switch amenity {
		case "restaurant", "fast_food", "food_court":
			return "restaurant"
		case "cafe", "coffee_shop":
			return "cafe"
		case "bar", "pub", "nightclub":
			return "bar"
		default:
			return "amenity"
		}
	}

	if tourism, ok := tags["tourism"]; ok {
		switch tourism {
		case "hotel", "motel", "hostel", "guest_house":
			return "hotel"
		case "attraction", "museum", "monument", "castle":
			return "attraction"
		default:
			return "tourism"
		}
	}

	if _, ok := tags["shop"]; ok {
		return "shopping"
	}

	if _, ok := tags["historic"]; ok {
		return "attraction"
	}

	return "place"
}

// buildAddress constructs a readable address from OSM tags
func (os *OverpassService) buildAddress(tags map[string]string, cityName string) string {
	var parts []string

	if housenumber, ok := tags["addr:housenumber"]; ok {
		parts = append(parts, housenumber)
	}

	if street, ok := tags["addr:street"]; ok {
		parts = append(parts, street)
	}

	if len(parts) == 0 && cityName != "" {
		parts = append(parts, cityName)
	}

	return strings.Join(parts, " ")
}

// inferPopularity estimates popularity based on OSM tags
func (os *OverpassService) inferPopularity(tags map[string]string) float64 {
	popularity := 0.5 // Base popularity

	// Tourism attractions tend to be more popular
	if _, ok := tags["tourism"]; ok {
		popularity += 0.3
	}

	// Historic places are often popular
	if _, ok := tags["historic"]; ok {
		popularity += 0.2
	}

	// Places with more detailed tagging are likely more significant
	if len(tags) > 5 {
		popularity += 0.1
	}

	// Cap at 1.0
	if popularity > 1.0 {
		popularity = 1.0
	}

	return popularity
}

// FixLocation attempts to get exact coordinates for a place using its approximate location and name
// Generates a 2km bounding box around the approximate coordinates for precise searching
func (os *OverpassService) FixLocation(placeName, cityName string, approxLat, approxLon float64) (float64, float64) {
	fmt.Printf("Fixing location for '%s' (approx: %.6f, %.6f)\n", placeName, approxLat, approxLon)

	// Rate limiting compliance
	os.rateLimiter.wait()

	// Generate a 2km bounding box around the approximate coordinates (1km in each direction)
	const searchRadiusKm = 1.0 // 1km radius = 2km total area

	// Convert 1km to degrees (rough approximation)
	// 1 degree latitude ≈ 111km, so 1km ≈ 0.009 degrees
	// Longitude depends on latitude, but we'll use a reasonable approximation
	latOffset := searchRadiusKm / 111.0                                     // ~0.009 degrees
	lonOffset := searchRadiusKm / (111.0 * math.Cos(approxLat*math.Pi/180)) // Adjust for latitude

	south := approxLat - latOffset
	north := approxLat + latOffset
	west := approxLon - lonOffset
	east := approxLon + lonOffset

	fmt.Printf("Searching in 2km box: %.6f,%.6f to %.6f,%.6f\n", south, west, north, east)

	// Try the precise bbox query first
	query := os.buildOverpassQueryWithBBox(placeName, south, west, north, east)
	response, err := os.executeQueryWithRetry(query, 2)

	if err != nil {
		fmt.Printf("Bbox query failed for '%s': %v\n", placeName, err)
		fmt.Printf("Falling back to simple global search...\n")

		// Fallback to simpler query
		simpleQuery := os.buildSimpleQuery(placeName, cityName)
		response, err = os.executeQueryWithRetry(simpleQuery, 2)
		if err != nil {
			fmt.Printf("Simple query also failed for '%s': %v, using approximate coordinates\n", placeName, err)
			return approxLat, approxLon
		}
	}

	// Parse and find the best match (with geographic preference for the bbox results)
	place, err := os.parseResponseWithLocation(response, placeName, approxLat, approxLon)
	if err != nil {
		fmt.Printf("Parsing failed for '%s': %v, using approximate coordinates\n", placeName, err)
		return approxLat, approxLon
	}

	// Check if the found location is reasonably close to the approximate location
	distance := os.calculateDistance(approxLat, approxLon, place.Lat, place.Lon)
	maxDistanceKm := 5.0 // Since we're using a 2km search box, anything beyond 5km is suspicious

	if distance > maxDistanceKm {
		fmt.Printf("Found location for '%s' is %.1fkm away from approximate location, might be different place. Using approximate coordinates\n", placeName, distance)
		return approxLat, approxLon
	}

	fmt.Printf("✓ Fixed location for '%s': %.6f, %.6f (was %.1fkm away)\t previous: %.6f, %.6f\n", placeName, place.Lat, place.Lon, distance, approxLat, approxLon)
	return place.Lat, place.Lon
}

// calculateDistance calculates the distance between two coordinates in kilometers using Haversine formula
func (os *OverpassService) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	// Convert degrees to radians
	lat1Rad := lat1 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	// Haversine formula
	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	sinLatHalf := math.Sin(dLat / 2)
	sinLonHalf := math.Sin(dLon / 2)

	a := sinLatHalf*sinLatHalf + math.Cos(lat1Rad)*math.Cos(lat2Rad)*sinLonHalf*sinLonHalf
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

// NormalizeName normalizes place names to handle common abbreviations and variations (public method)
func (os *OverpassService) NormalizeName(name string) string {
	return os.normalizeName(name)
}

// normalizeName normalizes place names to handle common abbreviations and variations
func (os *OverpassService) normalizeName(name string) string {
	if name == "" {
		return ""
	}

	// Convert to lowercase for consistent processing
	normalized := strings.ToLower(strings.TrimSpace(name))

	// Handle common abbreviations and variations
	replacements := map[string]string{
		// Saint/St variations
		"st.":  "saint",
		"st ":  "saint ",
		"ste.": "sainte",
		"ste ": "sainte ",

		// Doctor/Dr variations
		"dr.": "doctor",
		"dr ": "doctor ",

		// Church/Cathedral variations
		"ch.":   "church",
		"cath.": "cathedral",

		// Avenue/Street variations
		"ave.":  "avenue",
		"ave ":  "avenue ",
		"str.":  "street",
		"rd.":   "road",
		"blvd.": "boulevard",
		"blvd ": "boulevard ",

		// Mount/Mountain variations
		"mt.":  "mount",
		"mt ":  "mount ",
		"mtn.": "mountain",
		"mtn ": "mountain ",

		// Fort variations
		"ft.": "fort",
		"ft ": "fort ",

		// Other common abbreviations
		"univ.": "university",
		"coll.": "college",
		"hosp.": "hospital",
		"mus.":  "museum",
		"natl.": "national",
		"intl.": "international",
		"co.":   "company",
		"corp.": "corporation",
		"inc.":  "incorporated",
		"ltd.":  "limited",
	}

	// Apply replacements
	for abbrev, full := range replacements {
		// Replace at the beginning of string
		if strings.HasPrefix(normalized, abbrev) {
			normalized = full + normalized[len(abbrev):]
		}
		// Replace in the middle (with space boundaries)
		normalized = strings.ReplaceAll(normalized, " "+abbrev+" ", " "+full+" ")
		// Replace at the end
		if strings.HasSuffix(normalized, " "+abbrev) {
			normalized = normalized[:len(normalized)-len(abbrev)] + full
		}
	}

	// Clean up extra spaces
	normalized = strings.Join(strings.Fields(normalized), " ")

	return normalized
}

// parseResponseWithLocation is like parseResponse but prioritizes matches closer to expected location
func (os *OverpassService) parseResponseWithLocation(response *OverpassResponse, placeName string, expectedLat, expectedLon float64) (*types.Place, error) {
	if len(response.Elements) == 0 {
		return nil, fmt.Errorf("no places found matching '%s'", placeName)
	}

	fmt.Printf("Found %d elements from Overpass for '%s', prioritizing by proximity to %.6f,%.6f\n",
		len(response.Elements), placeName, expectedLat, expectedLon)

	// Find the best match - prioritize by name match + geographic proximity
	var bestMatch *OverpassElement
	var bestScore float64

	for i, element := range response.Elements {
		// Skip elements without coordinates (check both direct coordinates and center)
		hasCoords := false
		var lat, lon float64

		if element.Lat != nil && element.Lon != nil {
			// Node coordinates
			hasCoords = true
			lat, lon = *element.Lat, *element.Lon
		} else if element.Center != nil {
			// Way/relation center coordinates
			hasCoords = true
			lat, lon = element.Center.Lat, element.Center.Lon
		}

		if !hasCoords {
			continue
		}

		// Skip elements without name tags
		if element.Tags == nil {
			continue
		}

		name := element.Tags["name"]
		englishName := element.Tags["name:en"]

		// Allow elements with either main name or English name
		if name == "" && englishName == "" {
			continue
		}

		// Use the name that we have for scoring (prefer main name, fall back to English)
		nameForScoring := name
		if nameForScoring == "" {
			nameForScoring = englishName
		}

		// Calculate name match score
		nameScore := os.calculateMatchScore(nameForScoring, placeName, &element)

		// Calculate distance penalty (closer = better score)
		distance := os.calculateDistance(expectedLat, expectedLon, lat, lon)

		// Distance score: exponential decay, heavily favor closer matches
		// Within 10km = full points, 50km = half points, 100km+ = very low points
		var distanceScore float64
		if distance <= 10 {
			distanceScore = 100
		} else if distance <= 50 {
			distanceScore = 100 * (50 - distance) / 40 // Linear decay from 100 to 0 between 10-50km
		} else if distance <= 100 {
			distanceScore = 20 * (100 - distance) / 50 // Linear decay from 20 to 0 between 50-100km
		} else {
			distanceScore = 5 // Very low score for distant matches
		}

		// Combined score: name match + distance bonus
		totalScore := float64(nameScore) + distanceScore

		fmt.Printf("Element %d (%s): name_score=%d, distance=%.1fkm, distance_score=%.1f, total=%.1f\n",
			i+1, name, nameScore, distance, distanceScore, totalScore)

		if bestMatch == nil || totalScore > bestScore {
			elementCopy := element
			bestMatch = &elementCopy
			bestScore = totalScore
			fmt.Printf("New best match: '%s' with combined score %.1f\n", name, totalScore)
		}
	}

	if bestMatch == nil {
		return nil, fmt.Errorf("no suitable match found for '%s' (checked %d elements)", placeName, len(response.Elements))
	}

	fmt.Printf("Final best match for '%s': %s (combined score: %.1f)\n", placeName, bestMatch.Tags["name"], bestScore)

	// Convert to Place
	place := os.convertToPlace(bestMatch, "")
	return place, nil
}
