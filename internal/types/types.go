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

// PlaceDetail represents detailed place information including sources
type PlaceDetail struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Address    string    `json:"address,omitempty"`
	AreaKey    string    `json:"area_key"`
	Popularity float64   `json:"popularity"`
	Sources    []Source  `json:"sources"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Source represents a data source for a place
type Source struct {
	Source   string `json:"source"`
	SourceID string `json:"source_id"`
	URL      string `json:"url"`
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

// ChildArea represents a child area for hierarchical navigation
type ChildArea struct {
	AreaKey string      `json:"area_key"`
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Center  Coordinate  `json:"center"`
	BBox    BoundingBox `json:"bbox"`
	Teaser  []Place     `json:"teaser"`
}

// Admin API Types

// BootstrapRequest represents a request to bootstrap/refresh an area
type BootstrapRequest struct {
	AreaKey string   `json:"area_key" binding:"required"`
	Cats    []string `json:"cats"`
	Force   bool     `json:"force,omitempty"`
}

// BootstrapResponse represents the response from a bootstrap request
type BootstrapResponse struct {
	Status  string `json:"status"`
	AreaKey string `json:"area_key"`
	JobID   string `json:"job_id"`
}

// AreaStatus represents the status of an area's cache and refresh state
type AreaStatus struct {
	AreaKey       string         `json:"area_key"`
	LastRefreshAt *time.Time     `json:"last_refresh_at"`
	PlacesCount   map[string]int `json:"places_count"`
	Stale         bool           `json:"stale"`
	LastJob       *JobStatus     `json:"last_job"`
}

// JobStatus represents the status of a background job
type JobStatus struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	DurationMS int    `json:"duration_ms"`
}

type QueryLocation struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

// AI Planning Types

// PlanRequest represents a request to generate a travel plan
type PlanRequest struct {
	Area        string   `json:"area" binding:"required"`     // Area key or description
	Duration    int      `json:"duration" binding:"required"` // Duration in days
	Interests   []string `json:"interests,omitempty"`         // User interests/preferences
	Budget      string   `json:"budget,omitempty"`            // Budget level (low, medium, high)
	TravelStyle string   `json:"travel_style,omitempty"`      // Travel style (adventure, relaxation, culture, etc.)
}

// PlanResponse represents the AI-generated travel plan
type PlanResponse struct {
	ID          string    `json:"id"`
	Area        string    `json:"area"`
	Duration    int       `json:"duration"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Days        []PlanDay `json:"days"`
	Tips        []string  `json:"tips,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
}

// PlanDay represents a single day in the travel plan
type PlanDay struct {
	Day         int            `json:"day"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Activities  []PlanActivity `json:"activities"`
}

// PlanActivity represents a single activity in the travel plan
type PlanActivity struct {
	Time        string `json:"time"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location,omitempty"`
	Category    string `json:"category,omitempty"`
	Duration    string `json:"duration,omitempty"`
	Cost        string `json:"cost,omitempty"`
}

// ChatRequest represents a chat request for travel advice
type ChatRequest struct {
	Area     string `json:"area" binding:"required"`     // Area context
	Question string `json:"question" binding:"required"` // User question
	Context  string `json:"context,omitempty"`           // Additional context
}

// ChatResponse represents the AI chat response
type ChatResponse struct {
	Answer      string    `json:"answer"`
	Suggestions []string  `json:"suggestions,omitempty"`
	GeneratedAt time.Time `json:"generated_at"`
}

// AttractionItem represents a single attraction/place item from AI response
type AttractionItem struct {
	Type             string          `json:"type"`
	Name             string          `json:"name"`
	ShortDescription string          `json:"short_description"`
	Latitude         FlexibleFloat64 `json:"latitude"`
	Longitude        FlexibleFloat64 `json:"longitude"`
}

// AttractionResponse represents the structure of AI response for attractions
type AttractionResponse struct {
	Attractions []AttractionItem `json:"attractions"`
	Restaurants []AttractionItem `json:"restaurants"`
	Cafes       []AttractionItem `json:"cafes"`
	Bars        []AttractionItem `json:"bars"`
	Hotels      []AttractionItem `json:"hotels"`
}
