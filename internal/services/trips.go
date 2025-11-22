package services

import (
	"encoding/json"
	"fmt"
	"places_api/internal/types"
)

// TripService handles trip planner operations
type TripService struct {
	gqlClient *GraphQLClient
}

// NewTripService creates a new trip service instance
func NewTripService(gqlClient *GraphQLClient) *TripService {
	return &TripService{
		gqlClient: gqlClient,
	}
}

// GetTripByID retrieves a trip by its ID from the trips table with nested destinations
// Uses Supabase GraphQL to fetch trip with related trip_destinations and locations
func (s *TripService) GetTripByID(tripID string) (*types.Trip, error) {
	fmt.Printf("🔍 GetTripByID called with tripID: %s\n", tripID)

	variables := map[string]interface{}{
		"tripId": tripID,
	}

	gqlResp, err := s.gqlClient.Execute(GetTripQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL query: %w", err)
	}

	// Parse the nested response structure
	var result struct {
		TripsCollection types.GraphQLCollection[types.TripGraphQLNode] `json:"tripsCollection"`
	}

	if err := json.Unmarshal(gqlResp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL data: %w", err)
	}

	if len(result.TripsCollection.Edges) == 0 {
		return nil, fmt.Errorf("trip not found with id: %s", tripID)
	}

	// Convert GraphQL node to Trip struct
	trip := result.TripsCollection.Edges[0].Node.ToTrip()

	fmt.Printf("✅ Successfully fetched trip: %s with %d destinations\n", trip.Name, len(trip.Destinations))
	return trip, nil
}

// GetItineraryPreferencesByTripID retrieves itinerary preferences by trip ID
// Each trip has a unique set of preferences (enforced by unique_trip_preferences constraint)
func (s *TripService) GetItineraryPreferencesByTripID(tripID string) (*types.ItineraryPreferences, error) {
	fmt.Printf("🔍 GetItineraryPreferencesByTripID called with tripID: %s\n", tripID)

	variables := map[string]interface{}{
		"tripId": tripID,
	}

	gqlResp, err := s.gqlClient.Execute(GetPreferencesByTripIDQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL query: %w", err)
	}

	// Parse the response
	var result struct {
		ItineraryPreferencesCollection types.GraphQLCollection[types.ItineraryPreferences] `json:"itinerary_preferencesCollection"`
	}

	if err := json.Unmarshal(gqlResp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL data: %w", err)
	}

	if len(result.ItineraryPreferencesCollection.Edges) == 0 {
		return nil, fmt.Errorf("preferences not found for trip: %s", tripID)
	}

	preferences := &result.ItineraryPreferencesCollection.Edges[0].Node

	fmt.Printf("✅ Successfully fetched preferences for trip %s: style=%s, pace=%s, budget=%s\n",
		tripID, preferences.TravelStyle, preferences.Pace, preferences.BudgetLevel)
	return preferences, nil
}

// GetItineraryPreferencesByID retrieves itinerary preferences by ID
func (s *TripService) GetItineraryPreferencesByID(preferencesID string) (*types.ItineraryPreferences, error) {
	fmt.Printf("🔍 GetItineraryPreferencesByID called with preferencesID: %s\n", preferencesID)

	variables := map[string]interface{}{
		"id": preferencesID,
	}

	gqlResp, err := s.gqlClient.Execute(GetPreferencesByIDQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL query: %w", err)
	}

	// Parse the response
	var result struct {
		ItineraryPreferencesCollection types.GraphQLCollection[types.ItineraryPreferences] `json:"itinerary_preferencesCollection"`
	}

	if err := json.Unmarshal(gqlResp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL data: %w", err)
	}

	if len(result.ItineraryPreferencesCollection.Edges) == 0 {
		return nil, fmt.Errorf("preferences not found with id: %s", preferencesID)
	}

	preferences := &result.ItineraryPreferencesCollection.Edges[0].Node

	fmt.Printf("✅ Successfully fetched preferences %s: style=%s, pace=%s, budget=%s\n",
		preferencesID, preferences.TravelStyle, preferences.Pace, preferences.BudgetLevel)
	return preferences, nil
}

