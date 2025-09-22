package services

import (
	"fmt"
	"places_api/internal/ai"
	"places_api/internal/types"
	"places_api/internal/utils"
)

// AreaResolutionService handles area resolution logic
type AreaResolutionService struct {
	supabase  *SupabaseService
	nominatim *NominatimService
	aiPlanner *ai.AIPlannerService
}

// NewAreaResolutionService creates a new area resolution service
func NewAreaResolutionService(supabase *SupabaseService, nominatim *NominatimService, aiPlanner *ai.AIPlannerService) *AreaResolutionService {
	return &AreaResolutionService{
		supabase:  supabase,
		nominatim: nominatim,
		aiPlanner: aiPlanner,
	}
}

// ResolveAreaResult represents the result of area resolution
type ResolveAreaResult struct {
	Data     interface{}
	CacheAge int // Cache age in seconds
}

// ResolveArea resolves an area query with optional bootstrap
func (a *AreaResolutionService) ResolveArea(originalQuery string, multi bool, bootstrap bool) (*ResolveAreaResult, error) {
	// Sanitize the query for database lookup
	sanitizedQuery := utils.SanitizeQuery(originalQuery)

	// Try to get from database first
	if a.supabase != nil {
		result, err := a.supabase.ResolveArea(sanitizedQuery, multi)
		if err != nil {
			return nil, fmt.Errorf("database query failed: %v", err)
		}

		// Check if we got valid results
		if a.hasValidResults(result) {
			queryLocation := utils.BuildQueryLocation(originalQuery)

			// Handle bootstrap request if needed
			if bootstrap && a.aiPlanner != nil {
				// Extract area key from result
				areaKey := a.extractAreaKey(result)
				go a.aiPlanner.GetAreaTopAttractions(queryLocation.City, areaKey)
			}

			return &ResolveAreaResult{
				Data:     result,
				CacheAge: 86400, // Long cache for existing data
			}, nil
		}
	}

	// No results from database - query Nominatim
	queryLocation := utils.BuildQueryLocation(originalQuery)
	area, err := a.nominatim.GeocodeStructured(queryLocation)
	if err != nil {
		return nil, fmt.Errorf("location not found: %v", err)
	}

	// Save the result to database for future queries
	if a.supabase != nil {
		if saveErr := a.supabase.SaveArea(area); saveErr != nil {
			// Log error but don't fail the request
			fmt.Printf("Warning: Failed to save area to database: %v\n", saveErr)
		}
	}

	// Handle bootstrap request if requested
	if bootstrap && a.aiPlanner != nil {
		go a.aiPlanner.GetAreaTopAttractions(queryLocation.City, area.AreaKey)
	}

	return &ResolveAreaResult{
		Data:     area,
		CacheAge: 3600, // Shorter cache for new data
	}, nil
}

// hasValidResults checks if the database result contains valid data
func (a *AreaResolutionService) hasValidResults(result interface{}) bool {
	if area, ok := result.(types.Area); ok && area.AreaKey != "" {
		return true
	}
	if areas, ok := result.([]types.Area); ok && len(areas) > 0 && areas[0].AreaKey != "" {
		return true
	}
	return false
}

// extractAreaKey extracts the area key from the result for bootstrap
func (a *AreaResolutionService) extractAreaKey(result interface{}) string {
	if area, ok := result.(types.Area); ok {
		return area.AreaKey
	}
	if areas, ok := result.([]types.Area); ok && len(areas) > 0 {
		return areas[0].AreaKey
	}
	return ""
}
