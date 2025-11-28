package services

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"places_api/internal/types"
	"regexp"
	"strings"
)

type PhotonResponse struct {
	Type     string           `json:"type"`
	Features []PhotonLocation `json:"features"`
}

type PhotonLocation struct {
	Type       string `json:"type"`
	Properties struct {
		OsmType     string    `json:"osm_type"`
		OsmID       int       `json:"osm_id"`
		OsmKey      string    `json:"osm_key"`
		OsmValue    string    `json:"osm_value"`
		Type        string    `json:"type"`
		Postcode    string    `json:"postcode"`
		Countrycode string    `json:"countrycode"`
		Name        string    `json:"name"`
		Country     string    `json:"country"`
		City        string    `json:"city"`
		Street      string    `json:"street"`
		State       string    `json:"state"`
		County      string    `json:"county"`
		HouseNumber string    `json:"housenumber"`
		Extent      []float64 `json:"extent"` // Bounding box: [minLon, minLat, maxLon, maxLat]
	} `json:"properties"`
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
}

type PhotonService struct {
}

func NewPhotonService() *PhotonService {
	return &PhotonService{}
}

func (s *PhotonService) GetLocationData(locationName string, latitude float64, longitude float64) (*PhotonLocation, error) {
	photonBaseURL := "https://photon.komoot.io/api/"
	// Build the query parameters
	queryParams := url.Values{}
	queryParams.Set("q", locationName)
	queryParams.Set("lat", fmt.Sprintf("%f", latitude))
	queryParams.Set("lon", fmt.Sprintf("%f", longitude))
	queryParams.Set("limit", "1")
	queryParams.Set("lang", "en")

	// Build the Photon URL
	photonURL := fmt.Sprintf("%s?%s", photonBaseURL, queryParams.Encode())

	resp, err := http.Get(photonURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get location data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get location data: %v", resp.StatusCode)
	}

	// Parse the response
	var photonResponse PhotonResponse
	if err := json.NewDecoder(resp.Body).Decode(&photonResponse); err != nil {
		return nil, fmt.Errorf("failed to parse photon response: %v", err)
	}

	if len(photonResponse.Features) == 0 {
		return nil, fmt.Errorf("no results found for query")
	}

	return &photonResponse.Features[0], nil
}

// SearchLocation searches for a location by name only (without requiring coordinates)
// Returns the first matching result or error if not found
func (s *PhotonService) SearchLocation(locationName string) (*PhotonLocation, error) {
	photonBaseURL := "https://photon.komoot.io/api/"

	// Build the query parameters
	queryParams := url.Values{}
	queryParams.Set("q", locationName)
	queryParams.Set("limit", "1")
	queryParams.Set("lang", "en")

	// Build the Photon URL
	photonURL := fmt.Sprintf("%s?%s", photonBaseURL, queryParams.Encode())

	resp, err := http.Get(photonURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get location data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("photon API returned status %d", resp.StatusCode)
	}

	// Parse the response
	var photonResponse PhotonResponse
	if err := json.NewDecoder(resp.Body).Decode(&photonResponse); err != nil {
		return nil, fmt.Errorf("failed to parse photon response: %v", err)
	}

	if len(photonResponse.Features) == 0 {
		return nil, fmt.Errorf("no results found for query: %s", locationName)
	}

	return &photonResponse.Features[0], nil
}

// SanitizeLocationName cleans up location names for better geocoding results
// Handles patterns like "Near X", "Close to X", etc.
func (s *PhotonService) SanitizeLocationName(locationName string) string {
	// Remove leading/trailing whitespace
	sanitized := strings.TrimSpace(locationName)

	// Patterns to remove (case insensitive)
	patterns := []string{
		`^near\s+`,            // "Near X" -> "X"
		`^close\s+to\s+`,      // "Close to X" -> "X"
		`^around\s+`,          // "Around X" -> "X"
		`^nearby\s+`,          // "Nearby X" -> "X"
		`^next\s+to\s+`,       // "Next to X" -> "X"
		`^beside\s+`,          // "Beside X" -> "X"
		`^opposite\s+`,        // "Opposite X" -> "X"
		`^in\s+front\s+of\s+`, // "In front of X" -> "X"
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		sanitized = re.ReplaceAllString(sanitized, "")
	}

	// Remove parenthetical information that might confuse geocoding
	// e.g., "Eiffel Tower (optional)" -> "Eiffel Tower"
	re := regexp.MustCompile(`\s*\([^)]*\)\s*`)
	sanitized = re.ReplaceAllString(sanitized, " ")

	// Clean up extra whitespace
	sanitized = strings.Join(strings.Fields(sanitized), " ")

	return sanitized
}

// IsExploratoryActivity checks if an activity suggests area exploration vs specific point
func (s *PhotonService) IsExploratoryActivity(activityName string) bool {
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
func (s *PhotonService) IsAreaLocation(location *PhotonLocation) bool {
	// Relations (R) are typically areas
	if location.Properties.OsmType != "R" {
		return false
	}

	// Check if it's a city, district, neighborhood, etc.
	locType := location.Properties.Type
	return locType == "city" ||
		locType == "town" ||
		locType == "village" ||
		locType == "district" ||
		locType == "neighbourhood" ||
		locType == "suburb" ||
		locType == "quarter" ||
		locType == "borough"
}

// EnrichActivity enriches an activity with accurate location data from Photon
// Returns true if enrichment was successful, false otherwise
func (s *PhotonService) EnrichActivity(activity *types.ItineraryActivity) bool {
	return s.EnrichActivityWithContext(activity, nil)
}

// EnrichActivityWithContext enriches an activity with location data using destination context for better accuracy
// destinationContext should contain the destination city/country to constrain the search
func (s *PhotonService) EnrichActivityWithContext(activity *types.ItineraryActivity, destinationContext *types.TripDestination) bool {
	if activity.LocationName == "" {
		return false
	}

	// Sanitize the location name
	sanitizedName := s.SanitizeLocationName(activity.LocationName)
	if sanitizedName == "" {
		return false
	}

	// Build query with destination context for better geofencing
	searchQuery := sanitizedName
	if destinationContext != nil {
		// Append city/country to query for geographical constraint
		searchQuery = s.BuildContextualQuery(sanitizedName, destinationContext)
	}

	// Try to search with contextual query
	var location *PhotonLocation
	var err error

	// If we have destination coordinates, use them for location biasing
	if destinationContext != nil && destinationContext.Latitude != nil && destinationContext.Longitude != nil {
		location, err = s.GetLocationData(searchQuery, *destinationContext.Latitude, *destinationContext.Longitude)
	} else {
		location, err = s.SearchLocation(searchQuery)
	}

	if err != nil {
		// Fallback: try without context if contextual search failed
		if destinationContext != nil {
			location, err = s.SearchLocation(sanitizedName)
			if err != nil {
				// Last resort: try original name
				if sanitizedName != activity.LocationName {
					location, err = s.SearchLocation(activity.LocationName)
					if err != nil {
						return false
					}
				} else {
					return false
				}
			}
		} else {
			// If sanitization changed the name and search failed, try original
			if sanitizedName != activity.LocationName {
				location, err = s.SearchLocation(activity.LocationName)
				if err != nil {
					return false
				}
			} else {
				return false
			}
		}
	}

	// Validate result is within reasonable distance of destination (if context provided)
	if destinationContext != nil && destinationContext.Latitude != nil && destinationContext.Longitude != nil {
		if !s.ValidateLocationDistance(location, *destinationContext.Latitude, *destinationContext.Longitude) {
			return false
		}
	}

	// Determine if this should be an area or point activity
	isExploratory := s.IsExploratoryActivity(activity.Name)
	isArea := s.IsAreaLocation(location)
	hasBbox := len(location.Properties.Extent) == 4

	// If exploratory activity and we have bbox data, use area mode
	if (isExploratory || isArea) && hasBbox {
		// Set bounding box [minLon, minLat, maxLon, maxLat]
		minLon := location.Properties.Extent[0]
		minLat := location.Properties.Extent[1]
		maxLon := location.Properties.Extent[2]
		maxLat := location.Properties.Extent[3]

		activity.BboxMinLon = &minLon
		activity.BboxMinLat = &minLat
		activity.BboxMaxLon = &maxLon
		activity.BboxMaxLat = &maxLat
		activity.ActivityMode = "area"

		// Still set center point as fallback
		if len(location.Geometry.Coordinates) >= 2 {
			lon := location.Geometry.Coordinates[0]
			lat := location.Geometry.Coordinates[1]
			activity.Latitude = &lat
			activity.Longitude = &lon
		}
	} else {
		// Use point mode - extract coordinates (Photon returns [lon, lat])
		if len(location.Geometry.Coordinates) >= 2 {
			lon := location.Geometry.Coordinates[0]
			lat := location.Geometry.Coordinates[1]
			activity.Latitude = &lat
			activity.Longitude = &lon
			activity.ActivityMode = "point"
		} else {
			// No coordinates available
			activity.ActivityMode = "none"
		}
	}

	// Build full address
	address := s.FormatAddress(location)
	if address != "" {
		activity.Address = address
	}

	return true
}

// FormatAddress creates a human-readable address from Photon location data
func (s *PhotonService) FormatAddress(location *PhotonLocation) string {
	var parts []string

	props := location.Properties

	// Add name if available
	if props.Name != "" {
		parts = append(parts, props.Name)
	}

	// Add street and house number
	if props.Street != "" {
		street := props.Street
		if props.HouseNumber != "" {
			street = props.HouseNumber + " " + street
		}
		parts = append(parts, street)
	}

	// Add postcode and city
	cityPart := ""
	if props.Postcode != "" {
		cityPart = props.Postcode
	}
	if props.City != "" {
		if cityPart != "" {
			cityPart += " " + props.City
		} else {
			cityPart = props.City
		}
	}
	if cityPart != "" {
		parts = append(parts, cityPart)
	}

	// Add state if available (for US, etc.)
	if props.State != "" {
		parts = append(parts, props.State)
	}

	// Add country
	if props.Country != "" {
		parts = append(parts, props.Country)
	}

	return strings.Join(parts, ", ")
}

// EnrichItinerary enriches all activities in an itinerary with location data
// Returns statistics about successful enrichments
func (s *PhotonService) EnrichItinerary(itinerary *types.TripItineraryResponse) (int, int) {
	totalActivities := 0
	enrichedCount := 0

	for dayIdx := range itinerary.Itinerary.Days {
		day := &itinerary.Itinerary.Days[dayIdx]
		for actIdx := range day.Activities {
			activity := &day.Activities[actIdx]
			totalActivities++

			if s.EnrichActivity(activity) {
				enrichedCount++
			}
		}
	}

	return enrichedCount, totalActivities
}

// EnrichItineraryWithFallback enriches all activities using tiered geocoding services with destination context
// Tries Photon first (free), then LocationIQ (free tier), then gives up gracefully
// Returns (photonSuccess, locationiqSuccess, totalActivities)
func (s *PhotonService) EnrichItineraryWithFallback(itinerary *types.TripItineraryResponse, locationiqService *LocationIQService, trip *types.Trip) (int, int, int) {
	totalActivities := 0
	photonSuccess := 0
	locationiqSuccess := 0

	for dayIdx := range itinerary.Itinerary.Days {
		day := &itinerary.Itinerary.Days[dayIdx]

		// Determine which destination this day corresponds to
		destinationContext := s.GetDestinationForDay(day, trip)

		for actIdx := range day.Activities {
			activity := &day.Activities[actIdx]
			totalActivities++

			// Try Photon first (free) with destination context
			if s.EnrichActivityWithContext(activity, destinationContext) {
				photonSuccess++
				continue
			}

			// Photon failed, try LocationIQ if available (also with context)
			if locationiqService != nil && locationiqService.EnrichActivityWithContext(activity, destinationContext) {
				locationiqSuccess++
				continue
			}

			// Both failed - activity will be saved without coordinates
		}
	}

	return photonSuccess, locationiqSuccess, totalActivities
}

// GetDestinationForDay determines which trip destination corresponds to a given itinerary day
func (s *PhotonService) GetDestinationForDay(day *types.ItineraryDay, trip *types.Trip) *types.TripDestination {
	if trip == nil || len(trip.Destinations) == 0 {
		return nil
	}

	// If day has a destination field that matches one of the trip destinations, use it
	if day.Destination != "" {
		for i := range trip.Destinations {
			dest := &trip.Destinations[i]
			// Match by display name (case insensitive)
			if strings.EqualFold(dest.DisplayName, day.Destination) {
				return dest
			}
			// Also try matching just the city name
			if dest.Location != nil && strings.EqualFold(dest.Location.Name, day.Destination) {
				return dest
			}
		}
	}

	// Fallback: use first destination (common for single-destination trips)
	return &trip.Destinations[0]
}

// BuildContextualQuery creates a search query with geographical context
func (s *PhotonService) BuildContextualQuery(locationName string, destination *types.TripDestination) string {
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
// Returns true if distance is acceptable (within ~200km for cities, ~500km for countries)
func (s *PhotonService) ValidateLocationDistance(location *PhotonLocation, destLat, destLon float64) bool {
	if location == nil || len(location.Geometry.Coordinates) < 2 {
		return false
	}

	// Calculate distance between result and destination
	resultLon := location.Geometry.Coordinates[0]
	resultLat := location.Geometry.Coordinates[1]

	distance := s.HaversineDistance(destLat, destLon, resultLat, resultLon)

	// Threshold: 200km for most searches (catches wrong-continent results)
	// This is generous enough to handle suburbs and nearby cities
	maxDistanceKm := 200.0

	return distance <= maxDistanceKm
}

// HaversineDistance calculates the great-circle distance between two points (in kilometers)
func (s *PhotonService) HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
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
