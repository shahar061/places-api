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

// CreateAirportMajorCityPrompt creates a prompt for determining the major city for an airport
func (qb *QueryBuilder) CreateAirportMajorCityPrompt(airportName, regionName string) string {
	prompt := fmt.Sprintf(`Airport Information:
- Airport Name: %s
- Region/Location: %s

Determine the PRIMARY MAJOR CITY that this airport serves.`, airportName, regionName)

	return prompt
}

// GetAirportMajorCitySystemMessage returns the system message for airport major city queries
func (qb *QueryBuilder) GetAirportMajorCitySystemMessage() string {
	return `You are a geolocation and aviation assistant.

Your job:
Given information about an airport, determine the PRIMARY MAJOR CITY that the airport serves.
This is the city a traveler would typically say they are flying to, not necessarily the closest town by distance.

Rules:
- Always return a SINGLE JSON object, and NOTHING ELSE. No extra text, no markdown, no comments.
- The JSON MUST strictly follow this schema:
{
  "major_city": string,   // primary city served, in English, title-cased (e.g. "Milan", "New York")
  "country": string,      // country name in English (e.g. "Italy", "United States")
  "confidence": number,   // between 0 and 1 (e.g. 0.94)
  "reasoning": string,    // brief explanation of how you chose the city
  "notes": string         // optional extra info; can be empty ""
}

Decision guidelines:
- If the airport is branded with a city name (e.g. "Paris Charles de Gaulle", "Milan Bergamo", "Tokyo Narita"), that city is usually the major_city.
- If the airport name contains multiple cities (e.g. "Milan Bergamo Airport"), choose the city that passengers most commonly associate with the airport.
  For example, BGY is commonly marketed for Milan, so major_city = "Milan".
- If the airport is clearly a city local airport (e.g. "Abu Dhabi International Airport"), use that city (e.g. "Abu Dhabi").
- For military bases, remote islands, or small strips, use the nearest well-known city or the city most commonly associated with that airport in commercial travel contexts.
- If you are not sure, choose your best guess and lower the confidence value.
- If you genuinely cannot determine a plausible city, set:
  - "major_city": ""
  - "confidence": 0
  - and explain in "reasoning".

Output format requirements:
- Output MUST be valid JSON.
- Do NOT wrap the JSON in markdown.
- Do NOT include any additional keys besides: major_city, country, confidence, reasoning, notes.`
}
