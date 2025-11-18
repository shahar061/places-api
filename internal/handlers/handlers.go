package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"places_api/internal/config"
	"places_api/internal/logger"
	"places_api/internal/services"
	"places_api/internal/types"
	"places_api/internal/utils"

	"github.com/gin-gonic/gin"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	config        *config.Config
	supabase      *services.SupabaseService
	areaService   *services.AreaResolutionService
	jobService    *services.JobService
	natsService   *services.NATSService
	photonService *services.PhotonService
	aiService     AIService
	logger        *logger.Logger
}

// AIService interface defines the methods we need from the AI service
type AIService interface {
	GetAirportMajorCity(airportName, regionName string) (*types.AirportMajorCityResponse, error)
}

// New creates a new Handler instance
func New(cfg *config.Config, supabaseService *services.SupabaseService, natsService *services.NATSService, photonService *services.PhotonService, aiService AIService) *Handler {
	nominatim := services.NewNominatimService()
	areaService := services.NewAreaResolutionService(supabaseService, nominatim)

	var jobService *services.JobService
	if natsService != nil {
		jobService = services.NewJobService(supabaseService, natsService)
	}

	return &Handler{
		config:        cfg,
		supabase:      supabaseService,
		areaService:   areaService,
		jobService:    jobService,
		natsService:   natsService,
		photonService: photonService,
		aiService:     aiService,
		logger:        logger.WithComponent("handlers"),
	}
}

// HandleHealthcheck returns the health status of the service
// @Summary Health check
// @Description Get the health status of the Places API service
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "Service is healthy"
// @Router /healthcheck [get]
func (h *Handler) HandleHealthcheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "places_api",
	})
}

// Public API Endpoints

// HandleResolveArea handles GET /v1/areas/resolve
// @Summary Resolve area from text query
// @Description Turn free-text queries like "Rome, Lazio, Italy" into canonical area keys with geometry.
// The query is always in the format of "City, Region(State), Country".
// @Tags areas
// @Accept json
// @Produce json
// @Param q query string true "Free-text location query"
// @Param multi query bool false "Return multiple candidates instead of best match"
// @Param bootstrap query bool false "Queue background refresh for default categories"
// @Success 200 {object} types.Area "Single area result"
// @Success 200 {array} types.Area "Multiple area results when multi=true"
// @Failure 400 {object} map[string]string "Bad request"
// @Router /v1/areas/resolve [get]
func (h *Handler) HandleResolveArea(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	multi := c.Query("multi") == "true"
	bootstrap := c.Query("bootstrap") == "true"

	// Use area service if available, otherwise return mock response
	if h.areaService != nil {
		result, err := h.areaService.ResolveArea(query, multi, bootstrap)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "location not found"})
			return
		}

		// If bootstrap is requested and we have job service, enhance with job info
		if bootstrap && h.jobService != nil {
			enhancedResult, err := h.enhanceWithJobInfo(result.Data)
			if err != nil {
				// Log error but don't fail the request - return original data
				fmt.Printf("Warning: Failed to enhance with job info: %v\n", err)
				c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", result.CacheAge))
				c.JSON(http.StatusOK, result.Data)
				return
			}

			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", result.CacheAge))
			c.JSON(http.StatusOK, enhancedResult)
			return
		}

		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", result.CacheAge))
		c.JSON(http.StatusOK, result.Data)
		return
	}

	// Return error when area service is not available
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "area service is not available"})
}

// HandleGetTopPlaces handles GET /v1/places/top
// @Summary Get top places for area
// @Description Return ranked places for an area from cache
// @Tags places
// @Accept json
// @Produce json
// @Param area query string true "Canonical area key"
// @Param cats query string false "Comma-separated categories (attraction,restaurant,cafe,bar,hotel)"
// @Param limit query int false "Number of results (default: 50, max: 100)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Param lang query string false "Language preference"
// @Param group_by query string false "Group results by category"
// @Success 200 {object} map[string]interface{} "Top places response"
// @Failure 400 {object} map[string]string "Bad request"
// @Header 200 {string} X-Cache "Cache hit/miss status"
// @Header 200 {string} X-Refresh-Queued "Whether refresh was queued"
// @Router /v1/places/top [get]
func (h *Handler) HandleGetTopPlaces(c *gin.Context) {
	area := c.Query("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "query parameter 'area' is required",
		})
		return
	}

	// Normalize area key: convert to lowercase and normalize non-ASCII characters to ASCII
	// This ensures backward compatibility with old non-ASCII keys while new keys are stored as ASCII
	area = utils.NormalizeToASCII(strings.ToLower(area))

	_ = c.DefaultQuery("cats", "attraction,restaurant,cafe,bar,hotel") // TODO: implement category filtering
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	_ = c.Query("lang")     // TODO: implement localization
	_ = c.Query("group_by") // TODO: implement grouping

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Use Supabase service if available, otherwise return mock response
	if h.supabase != nil {
		places, err := h.supabase.GetTopPlaces(area, c.DefaultQuery("cats", "attraction,restaurant,cafe,bar,hotel"), limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to fetch top places",
			})
			return
		}

		response := gin.H{
			"area_key": area,
			"results":  places,
		}

		c.Header("X-Cache", "hit")
		c.Header("X-Refresh-Queued", "false")
		c.Header("Cache-Control", "public, max-age=300")
		c.JSON(http.StatusOK, response)
		return
	}

	response := gin.H{
		"area_key": area,
		"results":  []types.Place{},
	}

	c.Header("X-Cache", "miss")
	c.Header("X-Refresh-Queued", "false")
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, response)
}

// enhanceWithJobInfo enhances area response with job and data freshness information
func (h *Handler) enhanceWithJobInfo(areaData interface{}) (*types.ResolveAreaResponse, error) {
	// Extract area information
	var area types.Area
	switch v := areaData.(type) {
	case types.Area:
		area = v
	case *types.Area:
		if v == nil {
			return nil, fmt.Errorf("area pointer is nil")
		}
		area = *v
	case []types.Area:
		if len(v) == 0 {
			return nil, fmt.Errorf("no area data provided")
		}
		area = v[0] // Use first area for multi-result
	default:
		return nil, fmt.Errorf("invalid area data type: %T", v)
	}

	// Validate that we have a valid area with required fields
	if area.AreaKey == "" {
		return nil, fmt.Errorf("area has no area key")
	}

	// Check data freshness
	freshness, err := h.jobService.CheckDataFreshness(area.AreaKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check data freshness: %v", err)
	}

	fmt.Printf("Data freshness check for area %s: IsStale=%v, PlaceCount=%d\n", area.AreaKey, freshness.IsStale, freshness.PlaceCount)

	response := &types.ResolveAreaResponse{
		Area:          area,
		DataFreshness: *freshness,
	}

	// If data is stale, check for existing job or create new one
	if freshness.IsStale {
		fmt.Printf("Data is stale for area %s, checking if job should be created\n", area.AreaKey)
		shouldCreate, existingJob, err := h.jobService.ShouldCreateJob(area.AreaKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check job status: %v", err)
		}

		if existingJob != nil {
			fmt.Printf("Found existing job %s for area %s\n", existingJob.ID, area.AreaKey)
			// Return existing job info
			response.JobStatus = &types.JobInfo{
				JobID:       existingJob.ID,
				Status:      existingJob.Status,
				Progress:    existingJob.GetProgress(),
				CreatedAt:   existingJob.CreatedAt,
				StartedAt:   existingJob.StartedAt,
				CompletedAt: existingJob.CompletedAt,
				StatusURL:   fmt.Sprintf("/v1/jobs/%s/status", existingJob.ID),
			}
		} else if shouldCreate {
			fmt.Printf("Creating new job for area %s\n", area.AreaKey)
			// Create new job
			job, err := h.jobService.CreateFetchAttractionsJob(&area)
			if err != nil {
				fmt.Printf("Error creating job for area %s: %v\n", area.AreaKey, err)
				return nil, fmt.Errorf("failed to create job: %v", err)
			}
			fmt.Printf("Successfully created job %s for area %s\n", job.ID, area.AreaKey)

			response.JobStatus = &types.JobInfo{
				JobID:       job.ID,
				Status:      job.Status,
				Progress:    job.GetProgress(),
				CreatedAt:   job.CreatedAt,
				StartedAt:   job.StartedAt,
				CompletedAt: job.CompletedAt,
				StatusURL:   fmt.Sprintf("/v1/jobs/%s/status", job.ID),
			}
		}
	}

	return response, nil
}

// HandleJobStatus handles GET /v1/jobs/{jobId}/status
// @Summary Get job status
// @Description Get the current status and progress of a background job
// @Tags jobs
// @Accept json
// @Produce json
// @Param jobId path string true "Job ID"
// @Success 200 {object} types.JobInfo "Job status information"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Job not found"
// @Router /v1/jobs/{jobId}/status [get]
func (h *Handler) HandleJobStatus(c *gin.Context) {
	jobID := c.Param("jobId")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job ID is required"})
		return
	}

	if h.jobService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "job service is not available"})
		return
	}

	job, err := h.jobService.GetJobStatus(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	jobInfo := &types.JobInfo{
		JobID:       job.ID,
		Status:      job.Status,
		Progress:    job.GetProgress(),
		CreatedAt:   job.CreatedAt,
		StartedAt:   job.StartedAt,
		CompletedAt: job.CompletedAt,
		StatusURL:   fmt.Sprintf("/v1/jobs/%s/status", job.ID),
	}

	// Set appropriate cache headers based on job status
	if job.Status == types.JobStatusCompleted || job.Status == types.JobStatusFailed {
		c.Header("Cache-Control", "public, max-age=3600") // Cache completed jobs for 1 hour
	} else {
		c.Header("Cache-Control", "no-cache") // Don't cache pending/running jobs
	}

	c.JSON(http.StatusOK, jobInfo)
}

// HandleSearchPlacesByText handles GET /v1/places/search
// @Summary Search places by text
// @Description Search for places by text using Photon geocoding service
// @Tags places
// @Accept json
// @Produce json
// @Param q query string true "Text query to search for"
// @Param latitude query number false "Latitude for better search results"
// @Param longitude query number false "Longitude for better search results"
// @Success 200 {object} services.PhotonLocation "First search result from Photon"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "No results found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /v1/places/search [get]
func (h *Handler) HandleSearchPlaceByTextQuery(c *gin.Context) {
	// Get and validate query parameter
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	// Get latitude and longitude (optional)
	var latitude, longitude float64
	var err error

	latStr := c.Query("latitude")
	lonStr := c.Query("longitude")

	// If latitude is provided, validate it
	if latStr != "" {
		latitude, err = strconv.ParseFloat(latStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid latitude parameter"})
			return
		}
		// Validate latitude range
		if latitude < -90 || latitude > 90 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "latitude must be between -90 and 90"})
			return
		}
	}

	// If longitude is provided, validate it
	if lonStr != "" {
		longitude, err = strconv.ParseFloat(lonStr, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid longitude parameter"})
			return
		}
		// Validate longitude range
		if longitude < -180 || longitude > 180 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "longitude must be between -180 and 180"})
			return
		}
	}

	// If one coordinate is provided, both should be provided
	if (latStr != "" && lonStr == "") || (latStr == "" && lonStr != "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "both latitude and longitude must be provided together"})
		return
	}

	// Check if photon service is available
	if h.photonService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "photon service is not available"})
		return
	}

	// Default coordinates to 0 if not provided (photon will still work without them)
	if latStr == "" && lonStr == "" {
		latitude = 0
		longitude = 0
	}

	// Search photon for the location
	result, err := h.photonService.GetLocationData(query, latitude, longitude)
	if err != nil {
		// Check if it's a "no results" error
		if err.Error() == "no results found for query" {
			c.JSON(http.StatusNotFound, gin.H{"error": "no results found for query"})
			return
		}
		// Other errors are internal server errors
		h.logger.Error().Err(err).Msg("Failed to search photon")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search places"})
		return
	}

	// Return the first result
	c.JSON(http.StatusOK, result)
}

// HandleDetermineAirportMajorCity handles POST /v1/airports/major-city
// @Summary Determine major city for an airport
// @Description Determine the primary major city that an airport serves using AI
// @Tags airports
// @Accept json
// @Produce json
// @Param request body types.AirportMajorCityRequest true "Airport information"
// @Success 200 {object} types.AirportMajorCityResponse "Major city determination result"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 503 {object} map[string]string "Service unavailable"
// @Router /v1/airports/major-city [post]
func (h *Handler) HandleDetermineAirportMajorCity(c *gin.Context) {
	var req types.AirportMajorCityRequest

	// Bind and validate the request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	// Validate required fields (additional validation beyond binding)
	if req.AirportName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "airport_name is required",
		})
		return
	}

	if req.RegionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "region_name is required",
		})
		return
	}

	// Check if AI service is available
	if h.aiService == nil {
		h.logger.Error().Msg("AI service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI service is not available",
		})
		return
	}

	// Call AI service to determine major city
	h.logger.Info().
		Str("airport_name", req.AirportName).
		Str("region_name", req.RegionName).
		Msg("Determining airport major city")

	result, err := h.aiService.GetAirportMajorCity(req.AirportName, req.RegionName)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("airport_name", req.AirportName).
			Str("region_name", req.RegionName).
			Msg("Failed to determine airport major city")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to determine major city",
		})
		return
	}

	h.logger.Info().
		Str("airport_name", req.AirportName).
		Str("major_city", result.MajorCity).
		Str("country", result.Country).
		Float64("confidence", result.Confidence).
		Msg("Successfully determined airport major city")

	// Return the result
	c.JSON(http.StatusOK, result)
}
