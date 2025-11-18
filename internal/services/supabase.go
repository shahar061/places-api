package services

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"places_api/internal/config"
	"places_api/internal/types"
	"strings"
	"time"

	"github.com/supabase-community/postgrest-go"
)

// Configure HTTP transport with environment-aware TLS settings
func init() {
	// Check if we're in development mode
	isDevelopment := os.Getenv("GO_ENV") == "development" ||
		os.Getenv("GIN_MODE") == "debug" ||
		os.Getenv("PLACES_API_SKIP_TLS_VERIFY") == "true"

	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: isDevelopment, // Only skip in development
			MinVersion:         tls.VersionTLS12,
		},
	}

	if isDevelopment {
		fmt.Printf("⚠️  TLS certificate verification DISABLED (development mode)\n")
	} else {
		fmt.Printf("🔒 TLS certificate verification ENABLED (production mode)\n")
	}
}

const (
	areasTable         = "areas"
	placesTable        = "places"
	jobsTable          = "jobs"
	areaRefreshesTable = "area_refreshes"
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

	// Create HTTP client with environment-aware TLS configuration
	isDevelopment := os.Getenv("GO_ENV") == "development" ||
		os.Getenv("GIN_MODE") == "debug" ||
		os.Getenv("PLACES_API_SKIP_TLS_VERIFY") == "true"

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: isDevelopment, // Only skip in development
				MinVersion:         tls.VersionTLS12,
			},
		},
	}

	client := postgrest.NewClient(supabaseURL, "", map[string]string{
		"apikey":        cfg.SupabaseKey,
		"Authorization": "Bearer " + cfg.SupabaseKey,
	})

	return &SupabaseService{
		client:     client,
		baseURL:    supabaseURL,
		apiKey:     cfg.SupabaseKey,
		httpClient: httpClient,
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

// GetTopPlaces retrieves ranked places for an area from cache
func (s *SupabaseService) GetTopPlaces(areaKey string, categories string, limit, offset int) ([]types.Place, error) {
	if s.httpClient == nil {
		return nil, fmt.Errorf("supabase service not initialized")
	}

	fmt.Printf("Getting top places for area: %s, categories: %s, limit: %d, offset: %d\n", areaKey, categories, limit, offset)

	// Build query URL - search for places where area_key equals areaKey or id starts with areaKey
	queryURL := fmt.Sprintf("%s/places?select=*", s.baseURL)

	// URL encode the areaKey to safely handle any special characters in the query
	encodedAreaKey := url.QueryEscape(areaKey)

	// Filter by area_key or id starting with area key
	queryURL += fmt.Sprintf("&or=(area_key.eq.%s,id.like.%s%%2A)", encodedAreaKey, encodedAreaKey)

	// Filter by categories if provided
	if categories != "" {
		categoryList := strings.Split(categories, ",")
		categoryFilters := make([]string, len(categoryList))
		for i, cat := range categoryList {
			categoryFilters[i] = fmt.Sprintf("category.eq.%s", strings.TrimSpace(cat))
		}
		queryURL += fmt.Sprintf("&or=(%s)", strings.Join(categoryFilters, ","))
	}

	// Order by popularity descending
	queryURL += "&order=popularity.desc"

	// Apply limit and offset
	if limit > 0 {
		queryURL += fmt.Sprintf("&limit=%d", limit)
	}
	if offset > 0 {
		queryURL += fmt.Sprintf("&offset=%d", offset)
	}

	fmt.Printf("Querying: %s\n", queryURL)

	// Create request
	req, err := http.NewRequest("GET", queryURL, nil)
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

	fmt.Printf("Retrieved %d places for area %s\n", len(places), areaKey)
	return places, nil
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

// SaveAttractions saves attractions to the places table
func (s *SupabaseService) SaveAttractions(attractions *types.AttractionResponse, areaKey string) error {
	if attractions == nil {
		return nil
	}

	// Combine all attraction types into a single slice
	allAttractions := make([]types.AttractionItem, 0)
	allAttractions = append(allAttractions, attractions.Attractions...)
	allAttractions = append(allAttractions, attractions.Restaurants...)
	allAttractions = append(allAttractions, attractions.Cafes...)
	allAttractions = append(allAttractions, attractions.Bars...)
	allAttractions = append(allAttractions, attractions.Hotels...)

	if len(allAttractions) == 0 {
		return nil
	}

	// Convert attractions to database format
	placesData := make([]map[string]interface{}, len(allAttractions))
	for i, attraction := range allAttractions {
		placesData[i] = map[string]interface{}{
			"id":          attraction.ID,
			"name":        attraction.Name,
			"category":    attraction.Type,
			"description": attraction.ShortDescription,
			"lat":         float64(attraction.Latitude),
			"lon":         float64(attraction.Longitude),
			"address":     attraction.Address,
			"area_key":    areaKey,
			"osm_type":    attraction.OsmData.OsmType,
			"osm_id":      attraction.OsmData.OsmID,
			"osm_key":     attraction.OsmData.OsmKey,
			"osm_value":   attraction.OsmData.OsmValue,
			"updated_at":  time.Now(),
		}
	}

	// Upsert attractions to database
	_, _, err := s.client.From(placesTable).Upsert(placesData, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to save attractions to database: %v", err)
	}

	fmt.Printf("Successfully saved %d attractions to database for area %s\n", len(allAttractions), areaKey)
	return nil
}

// Job-related methods

// CreateJob creates a new job in the database
func (s *SupabaseService) CreateJob(job *types.Job) error {
	// Generate UUID for the job if not set
	if job.ID == "" {
		// Let Supabase generate the UUID
		job.ID = ""
	}

	jobData := map[string]interface{}{
		"area_key":   job.AreaKey,
		"job_type":   string(job.JobType),
		"status":     string(job.Status),
		"created_at": job.CreatedAt,
		"progress":   job.Progress,
		"metadata":   job.Metadata,
	}

	response, _, err := s.client.From(jobsTable).Insert(jobData, false, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to create job: %v", err)
	}

	// Parse response to get the generated ID
	var jobs []map[string]interface{}
	if err := json.Unmarshal(response, &jobs); err != nil {
		return fmt.Errorf("failed to parse job creation response: %v", err)
	}

	if len(jobs) > 0 {
		if id, ok := jobs[0]["id"].(string); ok {
			job.ID = id
		}
	}

	return nil
}

// GetJob retrieves a job by ID
func (s *SupabaseService) GetJob(jobID string) (*types.Job, error) {
	response, _, err := s.client.From(jobsTable).Select("*", "", false).Eq("id", jobID).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %v", err)
	}

	var jobs []map[string]interface{}
	if err := json.Unmarshal(response, &jobs); err != nil {
		return nil, fmt.Errorf("failed to parse job response: %v", err)
	}

	if len(jobs) == 0 {
		return nil, fmt.Errorf("job not found")
	}

	return s.mapToJob(jobs[0])
}

// GetLatestJobForArea retrieves the most recent job for an area
func (s *SupabaseService) GetLatestJobForArea(areaKey string) (*types.Job, error) {
	response, _, err := s.client.From(jobsTable).
		Select("*", "", false).
		Eq("area_key", areaKey).
		Order("created_at", &postgrest.OrderOpts{Ascending: false}).
		Limit(1, "").
		Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest job for area: %v", err)
	}

	var jobs []map[string]interface{}
	if err := json.Unmarshal(response, &jobs); err != nil {
		return nil, fmt.Errorf("failed to parse job response: %v", err)
	}

	if len(jobs) == 0 {
		return nil, fmt.Errorf("no jobs found for area")
	}

	return s.mapToJob(jobs[0])
}

// UpdateJobStatus updates a job's status and progress
func (s *SupabaseService) UpdateJobStatus(jobID string, status types.JobStatus, progress map[string]interface{}, errorMessage *string) error {
	// Get current job to check if started_at needs to be set
	currentJob, err := s.GetJob(jobID)
	if err != nil {
		// If we can't get the job, still proceed with the update
		// but we won't be able to set started_at conditionally
		currentJob = nil
	}

	updateData := map[string]interface{}{
		"status":   string(status),
		"progress": progress,
	}

	// Set started_at when transitioning to Running status (only if not already set)
	if status == types.JobStatusRunning {
		if currentJob == nil || currentJob.StartedAt == nil {
			updateData["started_at"] = time.Now()
		}
	}

	if status == types.JobStatusCompleted || status == types.JobStatusFailed {
		updateData["completed_at"] = time.Now()
	}

	if errorMessage != nil {
		updateData["error_message"] = *errorMessage
	}

	_, _, err = s.client.From(jobsTable).Update(updateData, "", "").Eq("id", jobID).Execute()
	if err != nil {
		return fmt.Errorf("failed to update job status: %v", err)
	}

	return nil
}

// mapToJob converts a database row to a Job struct
func (s *SupabaseService) mapToJob(data map[string]interface{}) (*types.Job, error) {
	job := &types.Job{}

	if id, ok := data["id"].(string); ok {
		job.ID = id
	}
	if areaKey, ok := data["area_key"].(string); ok {
		job.AreaKey = areaKey
	}
	if jobType, ok := data["job_type"].(string); ok {
		job.JobType = types.JobType(jobType)
	}
	if status, ok := data["status"].(string); ok {
		job.Status = types.JobStatus(status)
	}

	// Parse timestamps
	if createdAtStr, ok := data["created_at"].(string); ok {
		if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			job.CreatedAt = createdAt
		}
	}
	if startedAtStr, ok := data["started_at"].(string); ok && startedAtStr != "" {
		if startedAt, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
			job.StartedAt = &startedAt
		}
	}
	if completedAtStr, ok := data["completed_at"].(string); ok && completedAtStr != "" {
		if completedAt, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
			job.CompletedAt = &completedAt
		}
	}

	if errorMsg, ok := data["error_message"].(string); ok && errorMsg != "" {
		job.ErrorMessage = &errorMsg
	}

	if progress, ok := data["progress"].(map[string]interface{}); ok {
		job.Progress = progress
	}
	if metadata, ok := data["metadata"].(map[string]interface{}); ok {
		job.Metadata = metadata
	}

	return job, nil
}

// Area refresh methods

// GetAreaRefresh retrieves area refresh information
func (s *SupabaseService) GetAreaRefresh(areaKey string) (*types.AreaRefresh, error) {
	response, _, err := s.client.From(areaRefreshesTable).Select("*", "", false).Eq("area_key", areaKey).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get area refresh: %v", err)
	}

	var refreshes []map[string]interface{}
	if err := json.Unmarshal(response, &refreshes); err != nil {
		return nil, fmt.Errorf("failed to parse area refresh response: %v", err)
	}

	if len(refreshes) == 0 {
		return nil, fmt.Errorf("area refresh not found")
	}

	return s.mapToAreaRefresh(refreshes[0])
}

// UpsertAreaRefresh creates or updates area refresh information
func (s *SupabaseService) UpsertAreaRefresh(refresh *types.AreaRefresh) error {
	refreshData := map[string]interface{}{
		"area_key":             refresh.AreaKey,
		"refresh_requested_at": refresh.RefreshRequestedAt,
		"categories":           refresh.Categories,
		"place_count":          refresh.PlaceCount,
		"updated_at":           time.Now(),
	}

	if refresh.LastRefreshedAt != nil {
		refreshData["last_refreshed_at"] = *refresh.LastRefreshedAt
	}
	if refresh.DataExpiresAt != nil {
		refreshData["data_expires_at"] = *refresh.DataExpiresAt
	}

	_, _, err := s.client.From(areaRefreshesTable).Upsert(refreshData, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("failed to upsert area refresh: %v", err)
	}

	return nil
}

// UpdateAreaRefreshCompleted marks an area refresh as completed
func (s *SupabaseService) UpdateAreaRefreshCompleted(areaKey string, placeCount int) error {
	now := time.Now()
	expiresAt := now.Add(30 * 24 * time.Hour) // 30 days TTL

	// Try to get existing record to preserve refresh_requested_at
	existingRefresh, err := s.GetAreaRefresh(areaKey)
	refreshRequestedAt := now
	if err == nil && !existingRefresh.RefreshRequestedAt.IsZero() {
		refreshRequestedAt = existingRefresh.RefreshRequestedAt
	}

	// Prepare update data
	updateData := map[string]interface{}{
		"last_refreshed_at":    now,
		"refresh_requested_at": refreshRequestedAt,
		"data_expires_at":      expiresAt,
		"place_count":          placeCount,
		"updated_at":           now,
		"categories":           []string{"attraction", "restaurant", "cafe", "bar", "hotel"},
	}

	// Try to update first
	response, _, err := s.client.From(areaRefreshesTable).Update(updateData, "", "").Eq("area_key", areaKey).Execute()
	if err != nil {
		return fmt.Errorf("failed to update area refresh completion: %v", err)
	}

	// Check if any rows were updated
	var updatedRows []map[string]interface{}
	if err := json.Unmarshal(response, &updatedRows); err == nil && len(updatedRows) == 0 {
		// No rows updated, record doesn't exist - insert it
		insertData := map[string]interface{}{
			"area_key":             areaKey,
			"last_refreshed_at":    now,
			"refresh_requested_at": refreshRequestedAt,
			"data_expires_at":      expiresAt,
			"place_count":          placeCount,
			"updated_at":           now,
			"categories":           []string{"attraction", "restaurant", "cafe", "bar", "hotel"},
		}

		_, _, err = s.client.From(areaRefreshesTable).Insert(insertData, false, "", "", "").Execute()
		if err != nil {
			// If insert fails due to race condition (another process inserted it), try update again
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
				// Race condition - another process inserted it, try update again
				_, _, updateErr := s.client.From(areaRefreshesTable).Update(updateData, "", "").Eq("area_key", areaKey).Execute()
				if updateErr != nil {
					return fmt.Errorf("failed to update area refresh completion after race condition: %v", updateErr)
				}
				return nil
			}
			return fmt.Errorf("failed to insert area refresh completion: %v", err)
		}
	}

	return nil
}

// mapToAreaRefresh converts a database row to an AreaRefresh struct
func (s *SupabaseService) mapToAreaRefresh(data map[string]interface{}) (*types.AreaRefresh, error) {
	refresh := &types.AreaRefresh{}

	if id, ok := data["id"].(string); ok {
		refresh.ID = id
	}
	if areaKey, ok := data["area_key"].(string); ok {
		refresh.AreaKey = areaKey
	}
	if placeCount, ok := data["place_count"].(float64); ok {
		refresh.PlaceCount = int(placeCount)
	}

	// Parse categories array
	if categoriesData, ok := data["categories"].([]interface{}); ok {
		categories := make([]string, len(categoriesData))
		for i, cat := range categoriesData {
			if catStr, ok := cat.(string); ok {
				categories[i] = catStr
			}
		}
		refresh.Categories = categories
	}

	// Parse timestamps
	if lastRefreshedStr, ok := data["last_refreshed_at"].(string); ok && lastRefreshedStr != "" {
		if lastRefreshed, err := time.Parse(time.RFC3339, lastRefreshedStr); err == nil {
			refresh.LastRefreshedAt = &lastRefreshed
		}
	}
	if refreshRequestedStr, ok := data["refresh_requested_at"].(string); ok {
		if refreshRequested, err := time.Parse(time.RFC3339, refreshRequestedStr); err == nil {
			refresh.RefreshRequestedAt = refreshRequested
		}
	}
	if dataExpiresStr, ok := data["data_expires_at"].(string); ok && dataExpiresStr != "" {
		if dataExpires, err := time.Parse(time.RFC3339, dataExpiresStr); err == nil {
			refresh.DataExpiresAt = &dataExpires
		}
	}
	if createdAtStr, ok := data["created_at"].(string); ok {
		if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			refresh.CreatedAt = createdAt
		}
	}
	if updatedAtStr, ok := data["updated_at"].(string); ok {
		if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			refresh.UpdatedAt = updatedAt
		}
	}

	return refresh, nil
}

// FindLocation calls the Supabase edge function to find and validate a location
func (s *SupabaseService) FindLocation(term string, limit int) ([]types.ValidatedLocation, error) {
	if s.httpClient == nil {
		return nil, fmt.Errorf("supabase service not initialized")
	}

	// Extract base URL (remove /rest/v1 suffix if present)
	baseURL := strings.TrimSuffix(s.baseURL, "/rest/v1")

	// Build the edge function URL
	functionURL := fmt.Sprintf("%s/functions/v1/locations-finder", baseURL)

	// Prepare request body
	requestBody := map[string]interface{}{
		"term":  strings.ToLower(strings.TrimSpace(term)),
		"limit": limit,
	}

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Create request
	req, err := http.NewRequest("POST", functionURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

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
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("edge function returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var locations []types.ValidatedLocation
	if err := json.NewDecoder(resp.Body).Decode(&locations); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return locations, nil
}
