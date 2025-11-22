package services

// GraphQL query definitions for Supabase
// Following the same patterns as the Dart client implementation

// GetTripQuery retrieves a trip with nested destinations and locations
const GetTripQuery = `
	query GetTrip($tripId: UUID!) {
		tripsCollection(filter: {id: {eq: $tripId}}, first: 1) {
			edges {
				node {
					id
					user_id
					name
					start_date
					end_date
					notes
					created_at
					cover_key
					cover_photo_id
					cover_photo_urls
					cover_user_name
					cover_user_username
					cover_user_profile_url
					cover_download_location
					trip_destinationsCollection(orderBy: {order_index: AscNullsLast}) {
						edges {
							node {
								id
								trip_id
								location_id
								order_index
								display_name
								country
								latitude
								longitude
								start_day_index
								end_day_index
								locations {
									id
									name
									country
									country_code
									state_name
									state_code
									location_type
								}
							}
						}
					}
				}
			}
		}
	}
`

// GetPreferencesByTripIDQuery retrieves itinerary preferences by trip ID
const GetPreferencesByTripIDQuery = `
	query GetPreferencesByTripId($tripId: UUID!) {
		itinerary_preferencesCollection(filter: {trip_id: {eq: $tripId}}, first: 1) {
			edges {
				node {
					id
					trip_id
					user_id
					budget_level
					travel_style
					interests
					pace
					include_hotels
					include_restaurants
					include_activities
					must_visit_attractions
					additional_notes
					max_activities_per_day
					status
					error_message
					created_at
					updated_at
				}
			}
		}
	}
`

// GetPreferencesByIDQuery retrieves itinerary preferences by ID
const GetPreferencesByIDQuery = `
	query GetPreferencesById($id: UUID!) {
		itinerary_preferencesCollection(filter: {id: {eq: $id}}, first: 1) {
			edges {
				node {
					id
					trip_id
					user_id
					budget_level
					travel_style
					interests
					pace
					include_hotels
					include_restaurants
					include_activities
					must_visit_attractions
					additional_notes
					max_activities_per_day
					status
					error_message
					created_at
					updated_at
				}
			}
		}
	}
`

