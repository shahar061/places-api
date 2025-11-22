package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NullableTime handles database timestamps that may not have timezone info
type NullableTime struct {
	time.Time
}

// UnmarshalJSON handles parsing timestamps with or without timezone
func (nt *NullableTime) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	// Remove quotes from JSON string
	str := strings.Trim(string(data), `"`)
	if str == "" {
		return nil
	}

	// Try parsing with different formats
	formats := []string{
		time.RFC3339Nano,             // 2006-01-02T15:04:05.999999999Z07:00
		time.RFC3339,                 // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05.999999", // Without timezone
		"2006-01-02T15:04:05",        // Without timezone and microseconds
		"2006-01-02 15:04:05.999999", // Space separator
		"2006-01-02 15:04:05",        // Space separator, no microseconds
	}

	for _, format := range formats {
		if t, err := time.Parse(format, str); err == nil {
			nt.Time = t
			return nil
		}
	}

	return fmt.Errorf("unable to parse time: %s", str)
}

// MarshalJSON converts time to JSON
func (nt NullableTime) MarshalJSON() ([]byte, error) {
	if nt.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(nt.Time.Format(time.RFC3339))
}

// FlexibleFloat64 handles JSON unmarshaling of float64 values that may come as strings or numbers
type FlexibleFloat64 float64

// UnmarshalJSON handles parsing float64 values from both string and numeric JSON values
func (f *FlexibleFloat64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*f = 0
		return nil
	}

	// Try parsing as float64 first
	var floatValue float64
	if err := json.Unmarshal(data, &floatValue); err == nil {
		*f = FlexibleFloat64(floatValue)
		return nil
	}

	// Try parsing as string then converting to float64
	var stringValue string
	if err := json.Unmarshal(data, &stringValue); err == nil {
		if stringValue == "" {
			*f = 0
			return nil
		}

		parsedFloat, err := strconv.ParseFloat(stringValue, 64)
		if err != nil {
			return fmt.Errorf("unable to parse coordinate value '%s' as float64: %v", stringValue, err)
		}
		*f = FlexibleFloat64(parsedFloat)
		return nil
	}

	return fmt.Errorf("unable to parse coordinate value: %s", string(data))
}

// MarshalJSON converts FlexibleFloat64 to JSON
func (f FlexibleFloat64) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(f))
}

// Float64 returns the underlying float64 value
func (f FlexibleFloat64) Float64() float64 {
	return float64(f)
}

// Coordinate represents a geographic coordinate with latitude and longitude
type Coordinate struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// BoundingBox represents a geographic bounding box
type BoundingBox struct {
	SouthLat float64 `json:"south_lat"`
	NorthLat float64 `json:"north_lat"`
	WestLon  float64 `json:"west_lon"`
	EastLon  float64 `json:"east_lon"`
}

// Place represents a place with basic information
type Place struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	Address     string    `json:"address,omitempty"`
	Popularity  float64   `json:"popularity"`
	UpdatedAt   time.Time `json:"updated_at"`
	DistanceM   *int      `json:"distance_m,omitempty"` // Only for nearby searches
	Description string    `json:"description,omitempty"`
}

// Area represents a geographical area with metadata
type Area struct {
	AreaKey       string      `json:"area_key"`
	Name          string      `json:"name"`
	Type          string      `json:"type"` // country|region|city|locality|neighborhood
	CountryCode   string      `json:"country_code"`
	AdminLevel    int         `json:"admin_level"`
	Center        Coordinate  `json:"center"`
	BBox          BoundingBox `json:"bbox"`
	RefreshedAt   time.Time   `json:"refreshed_at"`
	RefreshQueued bool        `json:"refresh_queued"`
}

// AreaFlat represents the flat database structure for areas table
type AreaFlat struct {
	AreaKey       string        `json:"area_key"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	CountryCode   *string       `json:"country_code"`
	AdminLevel    *int          `json:"admin_level"`
	CenterLat     *float64      `json:"center_lat"`
	CenterLon     *float64      `json:"center_lon"`
	BboxSouth     *float64      `json:"bbox_south"`
	BboxNorth     *float64      `json:"bbox_north"`
	BboxWest      *float64      `json:"bbox_west"`
	BboxEast      *float64      `json:"bbox_east"`
	RefreshedAt   *NullableTime `json:"refreshed_at"`
	RefreshQueued *bool         `json:"refresh_queued"`
	CreatedAt     *NullableTime `json:"created_at"`
	UpdatedAt     *NullableTime `json:"updated_at"`
}

// ToFlat converts Area to AreaFlat for database operations
func (a *Area) ToFlat() *AreaFlat {
	flat := &AreaFlat{
		AreaKey: a.AreaKey,
		Name:    a.Name,
		Type:    a.Type,
	}

	if a.CountryCode != "" {
		flat.CountryCode = &a.CountryCode
	}
	if a.AdminLevel != 0 {
		flat.AdminLevel = &a.AdminLevel
	}
	if a.Center.Lat != 0 {
		flat.CenterLat = &a.Center.Lat
	}
	if a.Center.Lon != 0 {
		flat.CenterLon = &a.Center.Lon
	}
	if a.BBox.SouthLat != 0 {
		flat.BboxSouth = &a.BBox.SouthLat
	}
	if a.BBox.NorthLat != 0 {
		flat.BboxNorth = &a.BBox.NorthLat
	}
	if a.BBox.WestLon != 0 {
		flat.BboxWest = &a.BBox.WestLon
	}
	if a.BBox.EastLon != 0 {
		flat.BboxEast = &a.BBox.EastLon
	}
	if !a.RefreshedAt.IsZero() {
		flat.RefreshedAt = &NullableTime{a.RefreshedAt}
	}
	flat.RefreshQueued = &a.RefreshQueued

	return flat
}

// FromFlat converts AreaFlat to Area from database operations
func FromFlat(flat *AreaFlat) *Area {
	area := &Area{
		AreaKey: flat.AreaKey,
		Name:    flat.Name,
		Type:    flat.Type,
	}

	if flat.CountryCode != nil {
		area.CountryCode = *flat.CountryCode
	}
	if flat.AdminLevel != nil {
		area.AdminLevel = *flat.AdminLevel
	}
	if flat.CenterLat != nil {
		area.Center.Lat = *flat.CenterLat
	}
	if flat.CenterLon != nil {
		area.Center.Lon = *flat.CenterLon
	}
	if flat.BboxSouth != nil {
		area.BBox.SouthLat = *flat.BboxSouth
	}
	if flat.BboxNorth != nil {
		area.BBox.NorthLat = *flat.BboxNorth
	}
	if flat.BboxWest != nil {
		area.BBox.WestLon = *flat.BboxWest
	}
	if flat.BboxEast != nil {
		area.BBox.EastLon = *flat.BboxEast
	}
	if flat.RefreshedAt != nil {
		area.RefreshedAt = flat.RefreshedAt.Time
	}
	if flat.RefreshQueued != nil {
		area.RefreshQueued = *flat.RefreshQueued
	}

	return area
}

type QueryLocation struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

// AttractionItem represents a single attraction/place item from AI response
type AttractionItem struct {
	ID               string          `json:"id"`
	Type             string          `json:"type"`
	Name             string          `json:"name"`
	ShortDescription string          `json:"short_description"`
	Latitude         FlexibleFloat64 `json:"latitude"`
	Longitude        FlexibleFloat64 `json:"longitude"`
	Street           string          `json:"street"`
	Address          string          `json:"address"`
	HouseNumber      string          `json:"house_number"`
	OsmData          OsmData         `json:"osm_data"`
}

type OsmData struct {
	OsmType  string `json:"osm_type"`
	OsmID    int    `json:"osm_id"`
	OsmKey   string `json:"osm_key"`
	OsmValue string `json:"osm_value"`
}

// AttractionResponse represents the structure of AI response for attractions
type AttractionResponse struct {
	Attractions []AttractionItem `json:"attractions"`
	Restaurants []AttractionItem `json:"restaurants"`
	Cafes       []AttractionItem `json:"cafes"`
	Bars        []AttractionItem `json:"bars"`
	Hotels      []AttractionItem `json:"hotels"`
}

// Job-related types
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type JobType string

const (
	JobTypeFetchAttractions JobType = "fetch_attractions"
	JobTypePlanTrip         JobType = "plan_trip"
)

type Job struct {
	ID           string                 `json:"id"`
	AreaKey      string                 `json:"area_key"`
	JobType      JobType                `json:"job_type"`
	Status       JobStatus              `json:"status"`
	CreatedAt    time.Time              `json:"created_at"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	Progress     map[string]interface{} `json:"progress,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type JobProgress struct {
	Percentage int    `json:"percentage"`
	Message    string `json:"message"`
	Step       string `json:"step"`
	Total      int    `json:"total,omitempty"`
	Current    int    `json:"current,omitempty"`
}

type AreaRefresh struct {
	ID                 string     `json:"id"`
	AreaKey            string     `json:"area_key"`
	LastRefreshedAt    *time.Time `json:"last_refreshed_at,omitempty"`
	RefreshRequestedAt time.Time  `json:"refresh_requested_at"`
	DataExpiresAt      *time.Time `json:"data_expires_at,omitempty"`
	Categories         []string   `json:"categories"`
	PlaceCount         int        `json:"place_count"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type FetchAttractionsMessage struct {
	JobID       string    `json:"job_id"`
	Area        Area      `json:"area"`
	Categories  []string  `json:"categories"`
	RequestedAt time.Time `json:"requested_at"`
}

type PlanTripMessage struct {
	JobID         string    `json:"job_id"`
	TripID        string    `json:"trip_id"`
	PreferencesID string    `json:"preferences_id"`
	RequestedAt   time.Time `json:"requested_at"`
}

type JobStatusMessage struct {
	JobID     string                 `json:"job_id"`
	AreaKey   string                 `json:"area_key"`
	Status    JobStatus              `json:"status"`
	Progress  *JobProgress           `json:"progress,omitempty"`
	Error     *string                `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type ResolveAreaResponse struct {
	Area          Area          `json:"area"`
	JobStatus     *JobInfo      `json:"job_status,omitempty"`
	DataFreshness DataFreshness `json:"data_freshness"`
}

type JobInfo struct {
	JobID       string       `json:"job_id"`
	Status      JobStatus    `json:"status"`
	Progress    *JobProgress `json:"progress,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	StatusURL   string       `json:"status_url"`
}

type DataFreshness struct {
	LastRefreshed *time.Time `json:"last_refreshed,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsStale       bool       `json:"is_stale"`
	PlaceCount    int        `json:"place_count"`
}

// NATS Configuration
const (
	JobsStreamName           = "JOBS"
	SubjectFetchAttractions  = "jobs.fetch_attractions"
	SubjectPlanTrip          = "jobs.plan_trip"
	SubjectJobStatus         = "jobs.status"
	ConsumerFetchAttractions = "fetch-attractions-worker"
	ConsumerPlanTrip         = "plan-trip-worker"
)

// Helper methods
func (ar *AreaRefresh) IsExpired() bool {
	if ar.DataExpiresAt == nil {
		return true
	}
	return time.Now().After(*ar.DataExpiresAt)
}

func (ar *AreaRefresh) NeedsRefresh() bool {
	return ar.IsExpired() || ar.LastRefreshedAt == nil
}

func (j *Job) SetProgress(percentage int, message, step string) {
	if j.Progress == nil {
		j.Progress = make(map[string]interface{})
	}
	j.Progress["percentage"] = percentage
	j.Progress["message"] = message
	j.Progress["step"] = step
}

func (j *Job) GetProgress() *JobProgress {
	if j.Progress == nil {
		return nil
	}

	progress := &JobProgress{}
	if p, ok := j.Progress["percentage"].(float64); ok {
		progress.Percentage = int(p)
	}
	if m, ok := j.Progress["message"].(string); ok {
		progress.Message = m
	}
	if s, ok := j.Progress["step"].(string); ok {
		progress.Step = s
	}
	if t, ok := j.Progress["total"].(float64); ok {
		progress.Total = int(t)
	}
	if c, ok := j.Progress["current"].(float64); ok {
		progress.Current = int(c)
	}

	return progress
}

// Airport Major City types
type AirportMajorCityRequest struct {
	AirportName string `json:"airport_name" binding:"required"`
	RegionName  string `json:"region_name" binding:"required"`
}

type AirportMajorCityResponse struct {
	MajorCity         string             `json:"major_city"`
	Country           string             `json:"country"`
	Confidence        float64            `json:"confidence"`
	Reasoning         string             `json:"reasoning"`
	Notes             string             `json:"notes"`
	ValidatedLocation *ValidatedLocation `json:"validated_location,omitempty"`
}

// ValidatedLocation represents the validated location data from Supabase
type ValidatedLocation struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	LocationType string  `json:"location_type"`
	Country      string  `json:"country"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	CountryCode  string  `json:"country_code"`
	Popularity   int     `json:"popularity"`
	StateCode    string  `json:"state_code"`
	StateName    string  `json:"state_name"`
}

// Trip Planner types

// TripPlanRequest represents the request to create a trip itinerary
type TripPlanRequest struct {
	TripID        string `json:"trip_id" binding:"required"`
	PreferencesID string `json:"preferences_id" binding:"required"`
}

// TripPlanResponse represents the response from creating a trip itinerary
type TripPlanResponse struct {
	TripID      string                 `json:"trip_id"`
	Itinerary   map[string]interface{} `json:"itinerary,omitempty"` // Placeholder for AI-generated itinerary
	Status      string                 `json:"status"`
	Message     string                 `json:"message"`
	GeneratedAt time.Time              `json:"generated_at"`
}

// Trip represents a trip from the trips table
type Trip struct {
	ID                    string            `json:"id"`
	UserID                string            `json:"user_id"`
	Name                  string            `json:"name"`
	StartDate             string            `json:"start_date"` // Date format: YYYY-MM-DD
	EndDate               string            `json:"end_date"`   // Date format: YYYY-MM-DD
	Notes                 *string           `json:"notes,omitempty"`
	CreatedAt             time.Time         `json:"created_at"`
	CoverKey              *string           `json:"cover_key,omitempty"`
	CoverPhotoID          *string           `json:"cover_photo_id,omitempty"`
	CoverPhotoURLs        *json.RawMessage  `json:"cover_photo_urls,omitempty"` // JSONB
	CoverUserName         *string           `json:"cover_user_name,omitempty"`
	CoverUserUsername     *string           `json:"cover_user_username,omitempty"`
	CoverUserProfileURL   *string           `json:"cover_user_profile_url,omitempty"`
	CoverDownloadLocation *string           `json:"cover_download_location,omitempty"`
	Destinations          []TripDestination `json:"destinations,omitempty"` // Nested via GraphQL
}

// TripDestination represents a destination in a trip from the trip_destinations table
type TripDestination struct {
	ID            string             `json:"id"`
	TripID        string             `json:"trip_id"`
	LocationID    string             `json:"location_id"`
	OrderIndex    int16              `json:"order_index"`
	DisplayName   string             `json:"display_name"`
	Country       *string            `json:"country,omitempty"`
	Latitude      *float64           `json:"latitude,omitempty"`
	Longitude     *float64           `json:"longitude,omitempty"`
	StartDayIndex *int               `json:"start_day_index,omitempty"`
	EndDayIndex   *int               `json:"end_day_index,omitempty"`
	Location      *LocationReference `json:"location,omitempty"` // Nested via GraphQL
}

// LocationReference represents a location from the locations table (nested in trip destinations)
type LocationReference struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Country      string  `json:"country"`
	CountryCode  string  `json:"country_code"`
	StateName    *string `json:"state_name,omitempty"`
	StateCode    *string `json:"state_code,omitempty"`
	LocationType string  `json:"location_type"`
}

// GraphQL response types for Supabase GraphQL API

// GraphQLRequest represents a GraphQL query request
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL query response
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message string   `json:"message"`
	Path    []string `json:"path,omitempty"`
}

// GraphQL collection response wrapper (edges/node pattern)
type GraphQLEdge[T any] struct {
	Node T `json:"node"`
}

type GraphQLCollection[T any] struct {
	Edges []GraphQLEdge[T] `json:"edges"`
}

// GraphQL response types for trips
type TripGraphQLNode struct {
	ID                         string                                         `json:"id"`
	UserID                     string                                         `json:"user_id"`
	Name                       string                                         `json:"name"`
	StartDate                  string                                         `json:"start_date"`
	EndDate                    string                                         `json:"end_date"`
	Notes                      *string                                        `json:"notes"`
	CreatedAt                  time.Time                                      `json:"created_at"`
	CoverKey                   *string                                        `json:"cover_key"`
	CoverPhotoID               *string                                        `json:"cover_photo_id"`
	CoverPhotoURLs             *json.RawMessage                               `json:"cover_photo_urls"`
	CoverUserName              *string                                        `json:"cover_user_name"`
	CoverUserUsername          *string                                        `json:"cover_user_username"`
	CoverUserProfileURL        *string                                        `json:"cover_user_profile_url"`
	CoverDownloadLocation      *string                                        `json:"cover_download_location"`
	TripDestinationsCollection *GraphQLCollection[TripDestinationGraphQLNode] `json:"trip_destinationsCollection"`
}

type TripDestinationGraphQLNode struct {
	ID            string             `json:"id"`
	TripID        string             `json:"trip_id"`
	LocationID    string             `json:"location_id"`
	OrderIndex    int16              `json:"order_index"`
	DisplayName   string             `json:"display_name"`
	Country       *string            `json:"country"`
	Latitude      *float64           `json:"latitude"`
	Longitude     *float64           `json:"longitude"`
	StartDayIndex *int               `json:"start_day_index"`
	EndDayIndex   *int               `json:"end_day_index"`
	Locations     *LocationReference `json:"locations"` // Note: singular "locations" field name from Supabase GraphQL
}

// Convert GraphQL node to Trip struct
func (n *TripGraphQLNode) ToTrip() *Trip {
	trip := &Trip{
		ID:                    n.ID,
		UserID:                n.UserID,
		Name:                  n.Name,
		StartDate:             n.StartDate,
		EndDate:               n.EndDate,
		Notes:                 n.Notes,
		CreatedAt:             n.CreatedAt,
		CoverKey:              n.CoverKey,
		CoverPhotoID:          n.CoverPhotoID,
		CoverPhotoURLs:        n.CoverPhotoURLs,
		CoverUserName:         n.CoverUserName,
		CoverUserUsername:     n.CoverUserUsername,
		CoverUserProfileURL:   n.CoverUserProfileURL,
		CoverDownloadLocation: n.CoverDownloadLocation,
		Destinations:          []TripDestination{},
	}

	// Convert nested destinations
	if n.TripDestinationsCollection != nil {
		for _, edge := range n.TripDestinationsCollection.Edges {
			dest := TripDestination{
				ID:            edge.Node.ID,
				TripID:        edge.Node.TripID,
				LocationID:    edge.Node.LocationID,
				OrderIndex:    edge.Node.OrderIndex,
				DisplayName:   edge.Node.DisplayName,
				Country:       edge.Node.Country,
				Latitude:      edge.Node.Latitude,
				Longitude:     edge.Node.Longitude,
				StartDayIndex: edge.Node.StartDayIndex,
				EndDayIndex:   edge.Node.EndDayIndex,
				Location:      edge.Node.Locations,
			}
			trip.Destinations = append(trip.Destinations, dest)
		}
	}

	return trip
}

// ItineraryPreferences represents preferences from the itinerary_preferences table
type ItineraryPreferences struct {
	ID                   string    `json:"id"`
	TripID               string    `json:"trip_id"`
	UserID               string    `json:"user_id"`
	BudgetLevel          string    `json:"budget_level"`           // Values: "budget", "midRange", "luxury"
	TravelStyle          string    `json:"travel_style"`           // Values: "adventure", "relaxation", "culture", "food", "nightlife", "family"
	Interests            []string  `json:"interests"`              // Array of interests
	Pace                 string    `json:"pace"`                   // Values: "relaxed", "moderate", "fast"
	IncludeHotels        bool      `json:"include_hotels"`         // Default: true
	IncludeRestaurants   bool      `json:"include_restaurants"`    // Default: true
	IncludeActivities    bool      `json:"include_activities"`     // Default: true
	MustVisitAttractions []string  `json:"must_visit_attractions"` // Optional array
	AdditionalNotes      *string   `json:"additional_notes,omitempty"`
	MaxActivitiesPerDay  *int      `json:"max_activities_per_day,omitempty"`
	Status               string    `json:"status"` // Values: "pending", "processing", "completed", "failed"
	ErrorMessage         *string   `json:"error_message,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// TripItineraryResponse represents the AI-generated itinerary
type TripItineraryResponse struct {
	Itinerary TripItinerary `json:"itinerary"`
}

type TripItinerary struct {
	Summary string         `json:"summary"`
	Days    []ItineraryDay `json:"days"`
}

type ItineraryDay struct {
	Date        string              `json:"date"`
	Destination string              `json:"destination"`
	Activities  []ItineraryActivity `json:"activities"`
}

type ItineraryActivity struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	LocationName string `json:"location_name"`
	Address      string `json:"address,omitempty"` // Enriched by geocoding services

	// Point coordinates (for specific locations like "Eiffel Tower")
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`

	// Bounding box (for exploratory/area activities like "Explore Budapest")
	BboxMinLon *float64 `json:"bbox_min_lon,omitempty"`
	BboxMinLat *float64 `json:"bbox_min_lat,omitempty"`
	BboxMaxLon *float64 `json:"bbox_max_lon,omitempty"`
	BboxMaxLat *float64 `json:"bbox_max_lat,omitempty"`

	// Activity mode: "point" (specific location), "area" (bbox region), "none" (no location)
	ActivityMode string `json:"activity_mode"`

	AttractionType  string `json:"attraction_type"`
	DurationMinutes int    `json:"duration_minutes"`
	Notes           string `json:"notes,omitempty"`
}
