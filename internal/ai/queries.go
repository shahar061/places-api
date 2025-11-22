package ai

import (
	"fmt"
	"places_api/internal/types"
	"strings"
	"time"
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

// CreateTripItineraryPrompt creates an optimized prompt for generating a full trip itinerary
func (qb *QueryBuilder) CreateTripItineraryPrompt(trip *types.Trip, preferences *types.ItineraryPreferences) string {
	// Build destinations summary
	destSummary := qb.buildDestinationsSummary(trip.Destinations)

	// Calculate trip duration
	tripDays := qb.calculateTripDays(trip.StartDate, trip.EndDate)

	// Build preferences context
	prefsContext := qb.buildPreferencesContext(preferences)

	prompt := fmt.Sprintf(`Generate a detailed day-by-day itinerary for this trip:

TRIP: %s
DATES: %s to %s (%d days)
DESTINATIONS: %s

TRAVELER PREFERENCES:
%s

REQUIREMENTS:
- Create %d daily plans
- Balance %s activities with %s pace
- Consider %s budget
- Include must-visit: %s
- Max %d activities/day
%s

OUTPUT ONLY valid JSON (no markdown, no extra text):
{
  "itinerary": {
    "summary": "1-sentence trip overview",
    "days": [
      {
        "date": "YYYY-MM-DD",
        "destination": "city name",
        "activities": [
          {
            "name": "activity name",
            "description": "brief description (max 100 chars)",
            "start_time": "HH:MM",
            "end_time": "HH:MM",
            "location_name": "place name, city",
            "attraction_type": "attraction|restaurant|cafe|hotel|transport|other",
            "duration_minutes": number,
            "notes": "practical tip (optional)"
          }
        ]
      }
    ]
  }
}

Keep descriptions concise. Focus on well-known places.`,
		trip.Name,
		trip.StartDate,
		trip.EndDate,
		tripDays,
		destSummary,
		prefsContext,
		tripDays,
		qb.formatInterests(preferences.Interests),
		preferences.Pace,
		preferences.BudgetLevel,
		qb.formatMustVisit(preferences.MustVisitAttractions),
		qb.getMaxActivities(preferences.MaxActivitiesPerDay, preferences.Pace),
		qb.buildInclusionFlags(preferences),
	)

	return prompt
}

// GetTripItinerarySystemMessage returns the system message for itinerary generation
func (qb *QueryBuilder) GetTripItinerarySystemMessage() string {
	return `You are an expert travel planner. Create realistic, practical itineraries.

RULES:
1. Output ONLY valid JSON (no markdown, no comments)
2. Keep descriptions under 100 characters
3. Use realistic timing (account for travel, meals, breaks)
4. Suggest well-known, accessible places
5. Balance activity types throughout the day
6. Consider opening hours and typical visit durations
7. Group activities by proximity when possible
8. Include practical tips in notes when relevant

Quality over quantity. Better to have fewer well-planned activities than an overwhelming schedule.`
}

// Helper functions for building the prompt

func (qb *QueryBuilder) buildDestinationsSummary(destinations []types.TripDestination) string {
	if len(destinations) == 0 {
		return "No specific destinations"
	}

	var parts []string
	for _, dest := range destinations {
		// Include location name and optional date range
		destInfo := dest.DisplayName
		if dest.Country != nil && *dest.Country != "" {
			destInfo += fmt.Sprintf(", %s", *dest.Country)
		}
		if dest.StartDayIndex != nil && dest.EndDayIndex != nil {
			destInfo += fmt.Sprintf(" (days %d-%d)", *dest.StartDayIndex+1, *dest.EndDayIndex+1)
		}
		parts = append(parts, destInfo)
	}

	return strings.Join(parts, "; ")
}

func (qb *QueryBuilder) calculateTripDays(startDate, endDate string) int {
	// Simple day calculation (you may want to use time.Parse for accuracy)
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 1
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return 1
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 {
		return 1
	}
	return days
}

func (qb *QueryBuilder) buildPreferencesContext(prefs *types.ItineraryPreferences) string {
	var context strings.Builder

	context.WriteString(fmt.Sprintf("- Style: %s\n", prefs.TravelStyle))
	context.WriteString(fmt.Sprintf("- Pace: %s\n", prefs.Pace))
	context.WriteString(fmt.Sprintf("- Budget: %s\n", prefs.BudgetLevel))

	if len(prefs.Interests) > 0 {
		context.WriteString(fmt.Sprintf("- Interests: %s\n", strings.Join(prefs.Interests, ", ")))
	}

	if prefs.AdditionalNotes != nil && *prefs.AdditionalNotes != "" {
		// Truncate if too long to save tokens
		notes := *prefs.AdditionalNotes
		if len(notes) > 200 {
			notes = notes[:200] + "..."
		}
		context.WriteString(fmt.Sprintf("- Notes: %s\n", notes))
	}

	return context.String()
}

func (qb *QueryBuilder) formatInterests(interests []string) string {
	if len(interests) == 0 {
		return "varied"
	}
	return strings.Join(interests, ", ")
}

func (qb *QueryBuilder) formatMustVisit(attractions []string) string {
	if len(attractions) == 0 {
		return "none specified"
	}
	// Limit to first 5 to save tokens
	if len(attractions) > 5 {
		return strings.Join(attractions[:5], ", ") + "..."
	}
	return strings.Join(attractions, ", ")
}

func (qb *QueryBuilder) getMaxActivities(maxFromPrefs *int, pace string) int {
	if maxFromPrefs != nil {
		return *maxFromPrefs
	}
	// Default based on pace
	switch pace {
	case "relaxed":
		return 3
	case "fast":
		return 6
	default: // moderate
		return 4
	}
}

func (qb *QueryBuilder) buildInclusionFlags(prefs *types.ItineraryPreferences) string {
	var flags []string

	if !prefs.IncludeHotels {
		flags = append(flags, "- Exclude hotel recommendations")
	}
	if !prefs.IncludeRestaurants {
		flags = append(flags, "- Exclude restaurant suggestions")
	}
	if !prefs.IncludeActivities {
		flags = append(flags, "- Exclude activity suggestions")
	}

	if len(flags) == 0 {
		return ""
	}

	return "\n" + strings.Join(flags, "\n")
}
