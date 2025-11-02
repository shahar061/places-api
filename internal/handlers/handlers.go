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

	"github.com/gin-gonic/gin"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	config      *config.Config
	supabase    *services.SupabaseService
	areaService *services.AreaResolutionService
	jobService  *services.JobService
	natsService *services.NATSService
	logger      *logger.Logger
}

// New creates a new Handler instance
func New(cfg *config.Config, supabaseService *services.SupabaseService, natsService *services.NATSService) *Handler {
	nominatim := services.NewNominatimService()
	areaService := services.NewAreaResolutionService(supabaseService, nominatim)

	var jobService *services.JobService
	if natsService != nil {
		jobService = services.NewJobService(supabaseService, natsService)
	}

	return &Handler{
		config:      cfg,
		supabase:    supabaseService,
		areaService: areaService,
		jobService:  jobService,
		natsService: natsService,
		logger:      logger.WithComponent("handlers"),
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

	// Transform area key to lowercase
	area = strings.ToLower(area)

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

	response := &types.ResolveAreaResponse{
		Area:          area,
		DataFreshness: *freshness,
	}

	// If data is stale, check for existing job or create new one
	if freshness.IsStale {
		shouldCreate, existingJob, err := h.jobService.ShouldCreateJob(area.AreaKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check job status: %v", err)
		}

		if existingJob != nil {
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
			// Create new job
			job, err := h.jobService.CreateFetchAttractionsJob(&area)
			if err != nil {
				return nil, fmt.Errorf("failed to create job: %v", err)
			}

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
