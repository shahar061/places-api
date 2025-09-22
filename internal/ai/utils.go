package ai

import (
	"fmt"
	"places_api/internal/types"
	"strings"
	"time"
)

func RemoveMarkdownCodeBlocks(content string) string {
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	return content
}

func FlattenAttractionResponse(attractionResp types.AttractionResponse, areaKey string, city string) []types.Place {
	// Convert to Places format
	var places []types.Place

	// Helper function to convert AttractionItem to Place
	convertToPlace := func(item types.AttractionItem, category string) types.Place {
		return types.Place{
			ID:          fmt.Sprintf("%s_%s_%s", areaKey, strings.ToLower(strings.ReplaceAll(item.Name, " ", "_")), category),
			Name:        item.Name,
			Category:    category,
			Lat:         item.Latitude.Float64(),
			Lon:         item.Longitude.Float64(),
			Address:     fmt.Sprintf("%s, %s", item.Name, city),
			Popularity:  80.0, // Default high popularity for AI recommendations (out of 100)
			UpdatedAt:   time.Now(),
			Description: item.ShortDescription,
		}
	}

	// Convert all categories
	for _, item := range attractionResp.Attractions {
		places = append(places, convertToPlace(item, "attraction"))
	}
	for _, item := range attractionResp.Restaurants {
		places = append(places, convertToPlace(item, "restaurant"))
	}
	for _, item := range attractionResp.Cafes {
		places = append(places, convertToPlace(item, "cafe"))
	}
	for _, item := range attractionResp.Bars {
		places = append(places, convertToPlace(item, "bar"))
	}
	for _, item := range attractionResp.Hotels {
		places = append(places, convertToPlace(item, "hotel"))
	}

	return places
}
