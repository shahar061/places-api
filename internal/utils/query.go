package utils

import (
	"places_api/internal/types"
	"strings"
)

// SanitizeQuery transforms a free-text query into a canonical form suitable for area key lookup
func SanitizeQuery(query string) string {
	// Sanitize the query and transform it to a canonical form
	query = strings.TrimSpace(query)
	query = strings.ToLower(query)
	query = strings.ReplaceAll(query, " ", "_")
	query = strings.ReplaceAll(query, "-", "_")
	query = strings.ReplaceAll(query, ".", "_")
	query = strings.ReplaceAll(query, ",", "_")
	query = strings.ReplaceAll(query, "'", "")
	query = strings.ReplaceAll(query, "\"", "")

	return query
}

// BuildQueryLocation builds a query location from a query string (e.g. "Rome, Lazio, Italy") => {City: "Rome", Region: "Lazio", Country: "Italy"}
func BuildQueryLocation(query string) types.QueryLocation {
	split := strings.Split(query, ",")

	// Trim whitespace from all parts
	for i := range split {
		split[i] = strings.TrimSpace(split[i])
	}

	switch len(split) {
	case 1:
		return types.QueryLocation{
			City:    "",
			Region:  "",
			Country: split[0],
		}
	case 2:
		return types.QueryLocation{
			City:    split[0],
			Region:  "",
			Country: split[1],
		}
	case 3:
		return types.QueryLocation{
			City:    split[0],
			Region:  split[1],
			Country: split[2],
		}
	default:
		// Fallback for empty or too many parts
		return types.QueryLocation{
			City:    "",
			Region:  "",
			Country: "",
		}
	}
}
