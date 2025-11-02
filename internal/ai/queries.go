package ai

import (
	"fmt"
	"places_api/internal/types"
	"strings"
)

// QueryBuilder handles the construction of AI prompts and queries
type QueryBuilder struct{}

// NewQueryBuilder creates a new QueryBuilder instance
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{}
}

// CreateAttractionsPrompt creates a prompt for getting top attractions for an area
func (qb *QueryBuilder) CreateAttractionsPrompt(area *types.Area) string {
	locationDesc := qb.buildLocationDescription(area)

	prompt := fmt.Sprintf(`Please provide the top attractions, restaurants, cafes, bars, and hotels for the following location:

%s

Return the response as a JSON object with the following exact structure:
{
  "attractions": [
    {
      "type": "attraction",
      "name": "Name of the attraction",
      "short_description": "Brief description",
      "latitude": 0.0,
      "longitude": 0.0
    }
  ],
  "restaurants": [
    {
      "type": "restaurant",
      "name": "Name of the restaurant",
      "short_description": "Brief description",
      "latitude": 0.0,
      "longitude": 0.0
    }
  ],
  "cafes": [
    {
      "type": "cafe",
      "name": "Name of the cafe",
      "short_description": "Brief description",
      "latitude": 0.0,
      "longitude": 0.0
    }
  ],
  "bars": [
    {
      "type": "bar",
      "name": "Name of the bar",
      "short_description": "Brief description",
      "latitude": 0.0,
      "longitude": 0.0
    }
  ],
  "hotels": [
    {
      "type": "hotel",
      "name": "Name of the hotel",
      "short_description": "Brief description",
      "latitude": 0.0,
      "longitude": 0.0
    }
  ]
}

Please include:
- 5-10 top attractions (museums, landmarks, parks, etc.)
- 3-5 popular restaurants
- 2-3 notable cafes
- 2-3 popular bars/nightlife spots
- 3-5 recommended hotels

Make sure all coordinates are accurate and within the area. Provide only the JSON response, no additional text.`, locationDesc)

	return prompt
}

// GetAttractionsSystemMessage returns the system message for attractions queries
func (qb *QueryBuilder) GetAttractionsSystemMessage() string {
	return "You are a helpful travel assistant that provides information about top attractions in various locations. Always respond with valid JSON in the exact format requested."
}

// buildLocationDescription creates a formatted description of the area for use in prompts
func (qb *QueryBuilder) buildLocationDescription(area *types.Area) string {
	var locationDesc strings.Builder
	locationDesc.WriteString(fmt.Sprintf("Area: %s", area.Name))

	if area.Type != "" {
		locationDesc.WriteString(fmt.Sprintf(" (Type: %s)", area.Type))
	}

	if area.CountryCode != "" {
		locationDesc.WriteString(fmt.Sprintf(", Country: %s", area.CountryCode))
	}

	// Add coordinate information if available
	if area.Center.Lat != 0 && area.Center.Lon != 0 {
		locationDesc.WriteString(fmt.Sprintf(", Center: %.6f, %.6f", area.Center.Lat, area.Center.Lon))
	}

	return locationDesc.String()
}
