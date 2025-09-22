package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"places_api/internal/ai"
	"places_api/internal/config"
	"places_api/internal/services"
	"places_api/internal/types"

	"github.com/gin-gonic/gin"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	config      *config.Config
	supabase    *services.SupabaseService
	nominatim   *services.NominatimService
	aiPlanner   *ai.AIPlannerService
	areaService *services.AreaResolutionService
}

// New creates a new Handler instance
func New(cfg *config.Config, supabaseService *services.SupabaseService, aiPlannerService *ai.AIPlannerService) *Handler {
	nominatim := services.NewNominatimService()
	areaService := services.NewAreaResolutionService(supabaseService, nominatim, aiPlannerService)

	return &Handler{
		config:      cfg,
		supabase:    supabaseService,
		nominatim:   nominatim,
		aiPlanner:   aiPlannerService,
		areaService: areaService,
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

		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", result.CacheAge))
		c.JSON(http.StatusOK, result.Data)
		return
	}

	// Fallback mock response
	h.handleMockResponse(c, multi, bootstrap)
}

// handleMockResponse provides a fallback mock response when services are unavailable
func (h *Handler) handleMockResponse(c *gin.Context, multi bool, bootstrap bool) {
	mockArea := types.Area{
		AreaKey:       "it_rome",
		Name:          "Rome",
		Type:          "city",
		CountryCode:   "IT",
		AdminLevel:    8,
		Center:        types.Coordinate{Lat: 41.9028, Lon: 12.4964},
		BBox:          types.BoundingBox{SouthLat: 41.7, NorthLat: 42.0, WestLon: 12.3, EastLon: 12.7},
		RefreshedAt:   time.Now().Add(-24 * time.Hour),
		RefreshQueued: bootstrap,
	}

	c.Header("Cache-Control", "public, max-age=86400")

	if multi {
		c.JSON(http.StatusOK, []types.Area{mockArea})
	} else {
		c.JSON(http.StatusOK, mockArea)
	}
}

// HandleGetAreaChildren handles GET /v1/areas/children
// @Summary List child areas
// @Description Get child areas for hierarchical navigation (e.g., cities in a country)
// @Tags areas
// @Accept json
// @Produce json
// @Param parent query string true "Area key of parent area"
// @Param types query string false "Comma-separated child types to return"
// @Param limit query int false "Number of results to return (default: 20, max: 50)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Success 200 {object} map[string]interface{} "Children response"
// @Failure 400 {object} map[string]string "Bad request"
// @Router /v1/areas/children [get]
func (h *Handler) HandleGetAreaChildren(c *gin.Context) {
	parent := c.Query("parent")
	if parent == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "query parameter 'parent' is required",
		})
		return
	}

	_ = c.Query("types") // TODO: implement type filtering
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 50 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// TODO: Implement database query for child areas
	// For now, return mock response

	response := gin.H{
		"parent": parent,
		"children": []types.ChildArea{
			{
				AreaKey: "it_rome",
				Name:    "Rome",
				Type:    "city",
				Center:  types.Coordinate{Lat: 41.9, Lon: 12.5},
				BBox:    types.BoundingBox{SouthLat: 41.7, NorthLat: 42.0, WestLon: 12.3, EastLon: 12.7},
				Teaser: []types.Place{
					{
						ID:         "colosseum_1",
						Name:       "Colosseum",
						Category:   "attraction",
						Lat:        41.89,
						Lon:        12.49,
						Popularity: 98.4,
						UpdatedAt:  time.Now(),
					},
				},
			},
		},
	}

	c.JSON(http.StatusOK, response)
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

	// Fall back to mock response when Supabase is not available
	places := []types.Place{
		{
			ID:         "9f9a_c1",
			Name:       "Colosseum",
			Category:   "attraction",
			Lat:        41.8902,
			Lon:        12.4922,
			Address:    "Piazza del Colosseo, Rome",
			Popularity: 98.4,
			UpdatedAt:  time.Now().Add(-2 * time.Hour),
		},
	}

	response := gin.H{
		"area_key": area,
		"results":  places,
	}

	c.Header("X-Cache", "miss")
	c.Header("X-Refresh-Queued", "false")
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, response)
}

// HandleGetNearbyPlaces handles GET /v1/places/near
// @Summary Find nearby places
// @Description Fast radial search using PostGIS over cached places
// @Tags places
// @Accept json
// @Produce json
// @Param lat query number true "Latitude of center point"
// @Param lon query number true "Longitude of center point"
// @Param radius query int false "Search radius in meters (default: 1200, max: 5000)"
// @Param cats query string false "Comma-separated categories to filter"
// @Param limit query int false "Number of results (default: 40, max: 100)"
// @Param lang query string false "Language preference"
// @Success 200 {object} map[string]interface{} "Nearby places response"
// @Failure 400 {object} map[string]string "Bad request"
// @Router /v1/places/near [get]
func (h *Handler) HandleGetNearbyPlaces(c *gin.Context) {
	latStr := c.Query("lat")
	lonStr := c.Query("lon")

	if latStr == "" || lonStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "query parameters 'lat' and 'lon' are required",
		})
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid 'lat' parameter",
		})
		return
	}

	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid 'lon' parameter",
		})
		return
	}

	radiusStr := c.DefaultQuery("radius", "1200")
	radius, err := strconv.Atoi(radiusStr)
	if err != nil || radius <= 0 || radius > 5000 {
		radius = 1200
	}

	_ = c.Query("cats") // TODO: implement category filtering
	limitStr := c.DefaultQuery("limit", "40")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 40
	}

	_ = c.Query("lang") // TODO: implement localization

	// TODO: Implement PostGIS radial search
	// For now, return mock response

	places := []types.Place{
		{
			ID:         "ab12_de",
			Name:       "Trastevere Cafe",
			Category:   "cafe",
			Lat:        41.889,
			Lon:        12.472,
			Popularity: 62.1,
			UpdatedAt:  time.Now(),
			DistanceM:  &[]int{540}[0],
		},
	}

	response := gin.H{
		"center":   types.Coordinate{Lat: lat, Lon: lon},
		"radius_m": radius,
		"results":  places,
	}

	c.JSON(http.StatusOK, response)
}

// HandleSearchPlaces handles GET /v1/places/search
// @Summary Search places by name
// @Description Typeahead search over cached place names within a given area
// @Tags places
// @Accept json
// @Produce json
// @Param area query string true "Area key to search within"
// @Param q query string true "Search query text (prefix/fuzzy)"
// @Param cats query string false "Comma-separated categories to filter"
// @Param limit query int false "Number of results (default: 20, max: 50)"
// @Param lang query string false "Language preference"
// @Success 200 {object} map[string]interface{} "Search results"
// @Failure 400 {object} map[string]string "Bad request"
// @Router /v1/places/search [get]
func (h *Handler) HandleSearchPlaces(c *gin.Context) {
	area := c.Query("area")
	query := c.Query("q")

	if area == "" || query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "query parameters 'area' and 'q' are required",
		})
		return
	}

	_ = c.Query("cats") // TODO: implement category filtering
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 50 {
		limit = 20
	}

	_ = c.Query("lang") // TODO: implement localization

	// TODO: Implement fuzzy search over cached names
	// For now, return mock response

	places := []types.Place{
		{
			ID:         "musei_cap_1",
			Name:       "Musei Capitolini",
			Category:   "attraction",
			Lat:        41.892,
			Lon:        12.482,
			Popularity: 75.3,
			UpdatedAt:  time.Now(),
		},
	}

	response := gin.H{
		"area_key": area,
		"query":    query,
		"results":  places,
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetPlaceDetails handles GET /v1/places/{id}
// @Summary Get place details
// @Description Get detailed information for a specific place
// @Tags places
// @Accept json
// @Produce json
// @Param id path string true "Place ID"
// @Success 200 {object} types.PlaceDetail "Place details"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Place not found"
// @Router /v1/places/{id} [get]
func (h *Handler) HandleGetPlaceDetails(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "place id is required",
		})
		return
	}

	// Use Supabase service if available, otherwise return mock response
	if h.supabase != nil {
		place, err := h.supabase.GetPlaceDetails(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "place not found",
			})
			return
		}
		c.JSON(http.StatusOK, place)
		return
	}

	// Fall back to mock response when Supabase is not available
	if id == "9f9a_c1" {
		place := types.PlaceDetail{
			ID:         id,
			Name:       "Colosseum",
			Category:   "attraction",
			Lat:        41.8902,
			Lon:        12.4922,
			Address:    "Piazza del Colosseo, Rome",
			AreaKey:    "it_rome",
			Popularity: 98.4,
			Sources: []types.Source{
				{
					Source:   "osm",
					SourceID: "way/123456",
					URL:      "https://www.openstreetmap.org/way/123456",
				},
			},
			UpdatedAt: time.Now().Add(-2 * time.Hour),
		}
		c.JSON(http.StatusOK, place)
	} else {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "place not found",
		})
	}
}

// Admin API Endpoints

// HandleBootstrapArea handles POST /v1/admin/areas/bootstrap
// @Summary Bootstrap area cache
// @Description Queue background refresh/warm for an area's place cache
// @Tags admin
// @Accept json
// @Produce json
// @Param request body types.BootstrapRequest true "Bootstrap request"
// @Success 202 {object} types.BootstrapResponse "Bootstrap queued successfully"
// @Failure 400 {object} map[string]string "Bad request"
// @Router /v1/admin/areas/bootstrap [post]
func (h *Handler) HandleBootstrapArea(c *gin.Context) {
	var req types.BootstrapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// TODO: Queue background job for area bootstrap
	// For now, return mock response

	response := types.BootstrapResponse{
		Status:  "queued",
		AreaKey: req.AreaKey,
		JobID:   "job_" + strconv.FormatInt(time.Now().Unix(), 10),
	}

	c.JSON(http.StatusAccepted, response)
}

// HandleGetAreaStatus handles GET /v1/admin/areas/status
// @Summary Get area cache status
// @Description Get refresh status and cache statistics for an area
// @Tags admin
// @Accept json
// @Produce json
// @Param area query string true "Area key to check status for"
// @Success 200 {object} types.AreaStatus "Area status information"
// @Failure 400 {object} map[string]string "Bad request"
// @Router /v1/admin/areas/status [get]
func (h *Handler) HandleGetAreaStatus(c *gin.Context) {
	area := c.Query("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "query parameter 'area' is required",
		})
		return
	}

	// TODO: Get actual status from database
	// For now, return mock response

	lastRefresh := time.Now().Add(-2 * time.Hour)
	status := types.AreaStatus{
		AreaKey:       area,
		LastRefreshAt: &lastRefresh,
		PlacesCount: map[string]int{
			"attraction": 210,
			"restaurant": 200,
			"cafe":       150,
		},
		Stale: false,
		LastJob: &types.JobStatus{
			ID:         "job_12345",
			Status:     "ok",
			DurationMS: 8421,
		},
	}

	c.JSON(http.StatusOK, status)
}

// AI Planning Endpoints

// HandleGeneratePlan handles POST /v1/ai/plan
// @Summary Generate travel plan
// @Description Generate an AI-powered travel plan for a specific area
// @Tags ai
// @Accept json
// @Produce json
// @Param plan body types.PlanRequest true "Plan request"
// @Success 200 {object} types.PlanResponse "Generated travel plan"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /v1/ai/plan [post]
func (h *Handler) HandleGeneratePlan(c *gin.Context) {
	if h.aiPlanner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI planning service not available",
		})
		return
	}

	var req types.PlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// Validate required fields
	if req.Area == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "area is required",
		})
		return
	}
	if req.Duration <= 0 || req.Duration > 30 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "duration must be between 1 and 30 days",
		})
		return
	}

	// Generate the plan
	plan, err := h.aiPlanner.GeneratePlan(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to generate travel plan",
		})
		return
	}

	c.JSON(http.StatusOK, plan)
}

// HandleTravelChat handles POST /v1/ai/chat
// @Summary Travel advice chat
// @Description Get AI-powered travel advice and recommendations
// @Tags ai
// @Accept json
// @Produce json
// @Param chat body types.ChatRequest true "Chat request"
// @Success 200 {object} types.ChatResponse "AI travel advice"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /v1/ai/chat [post]
func (h *Handler) HandleTravelChat(c *gin.Context) {
	if h.aiPlanner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI planning service not available",
		})
		return
	}

	var req types.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request format",
		})
		return
	}

	// Validate required fields
	if req.Area == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "area is required",
		})
		return
	}
	if req.Question == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "question is required",
		})
		return
	}

	// Get AI response
	response, err := h.aiPlanner.ChatTravel(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get travel advice",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
