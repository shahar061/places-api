package types

import "time"

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
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Address    string    `json:"address,omitempty"`
	Popularity float64   `json:"popularity"`
	UpdatedAt  time.Time `json:"updated_at"`
	DistanceM  *int      `json:"distance_m,omitempty"` // Only for nearby searches
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
