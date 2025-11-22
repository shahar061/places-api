package worker

import (
	"fmt"
	"places_api/internal/ai"
	"places_api/internal/services"
	"places_api/internal/types"
)

// Worker handles background job processing
type Worker struct {
	jobService        *services.JobService
	natsService       *services.NATSService
	supabase          *services.SupabaseService
	aiService         *ai.Service
	photonService     *services.PhotonService
	locationiqService *services.LocationIQService
}

// NewWorker creates a new worker instance
func NewWorker(jobService *services.JobService, natsService *services.NATSService, supabase *services.SupabaseService, aiService *ai.Service, photonService *services.PhotonService, locationiqService *services.LocationIQService) *Worker {
	return &Worker{
		jobService:        jobService,
		natsService:       natsService,
		supabase:          supabase,
		aiService:         aiService,
		photonService:     photonService,
		locationiqService: locationiqService,
	}
}

// Start begins processing jobs from NATS
func (w *Worker) Start() error {
	fmt.Printf("Starting background worker...\n")

	// Subscribe to fetch attractions jobs
	err := w.natsService.SubscribeFetchAttractions(w.processFetchAttractionsJob)
	if err != nil {
		return fmt.Errorf("failed to subscribe to fetch attractions jobs: %v", err)
	}

	// Subscribe to plan trip jobs
	err = w.natsService.SubscribePlanTrip(w.processPlanTripJob)
	if err != nil {
		return fmt.Errorf("failed to subscribe to plan trip jobs: %v", err)
	}

	// Subscribe to job status updates (for logging/monitoring)
	err = w.natsService.SubscribeJobStatus(w.processJobStatusUpdate)
	if err != nil {
		return fmt.Errorf("failed to subscribe to job status updates: %v", err)
	}

	fmt.Printf("Background worker started and listening for jobs\n")
	return nil
}

// processFetchAttractionsJob processes a fetch attractions job
func (w *Worker) processFetchAttractionsJob(msg *types.FetchAttractionsMessage) error {
	fmt.Printf("📋 Processing fetch attractions job: %s for area: %s\n", msg.JobID, msg.Area.AreaKey)

	// Update job status to running
	err := w.jobService.UpdateJobProgress(msg.JobID, 0, "Starting job", "initializing")
	if err != nil {
		fmt.Printf("Error updating job progress: %v\n", err)
		return err
	}

	// Call the AI service to get top attractions for the area
	attractions, err := w.aiService.GetTopAttractions(&msg.Area)
	if err != nil {
		fmt.Printf("Error getting top attractions: %v\n", err)
		// Mark job as failed
		if markErr := w.jobService.MarkJobFailed(msg.JobID, fmt.Sprintf("Failed to get attractions from AI: %v", err)); markErr != nil {
			fmt.Printf("Error marking job as failed: %v\n", markErr)
		}
		return err
	}

	// For each attraction, call the Photon service to get the location data
	totalProcessed := 0
	totalProcessed += w.enrichAttractionsWithLocationData(attractions.Attractions, msg.Area.AreaKey)
	totalProcessed += w.enrichAttractionsWithLocationData(attractions.Restaurants, msg.Area.AreaKey)
	totalProcessed += w.enrichAttractionsWithLocationData(attractions.Cafes, msg.Area.AreaKey)
	totalProcessed += w.enrichAttractionsWithLocationData(attractions.Bars, msg.Area.AreaKey)
	totalProcessed += w.enrichAttractionsWithLocationData(attractions.Hotels, msg.Area.AreaKey)
	// Save all attractions to Supabase
	err = w.supabase.SaveAttractions(attractions, msg.Area.AreaKey)
	if err != nil {
		fmt.Printf("Error saving attractions to database: %v\n", err)
		// Mark job as failed
		if markErr := w.jobService.MarkJobFailed(msg.JobID, fmt.Sprintf("Failed to save attractions to database: %v", err)); markErr != nil {
			fmt.Printf("Error marking job as failed: %v\n", markErr)
		}
		return err
	}

	fmt.Printf("Successfully processed and saved %d attractions for area %s\n", totalProcessed, msg.Area.AreaKey)

	// Mark job as completed
	if err := w.jobService.CompleteJob(msg.JobID, totalProcessed); err != nil {
		fmt.Printf("Error completing job: %v\n", err)
		return err
	}

	return nil
}

func (w *Worker) enrichAttractionsWithLocationData(attractions []types.AttractionItem, areaKey string) int {
	for i := range len(attractions) {
		locationData, err := w.photonService.GetLocationData(attractions[i].Name, float64(attractions[i].Latitude), float64(attractions[i].Longitude))
		if err != nil {
			fmt.Printf("Error getting location data: %v\n", err)
			continue
		}

		attractions[i].Street = locationData.Properties.Street
		// Build address
		w.buildAddress(locationData, attractions, i)
		attractions[i].HouseNumber = locationData.Properties.HouseNumber
		attractions[i].OsmData = types.OsmData{
			OsmType:  locationData.Properties.OsmType,
			OsmID:    locationData.Properties.OsmID,
			OsmKey:   locationData.Properties.OsmKey,
			OsmValue: locationData.Properties.OsmValue,
		}

		// Build attraction id
		attractions[i].ID = fmt.Sprintf("%s_%s_%d", areaKey, attractions[i].Type, attractions[i].OsmData.OsmID)
	}

	return len(attractions)
}

func (*Worker) buildAddress(locationData *services.PhotonLocation, attractions []types.AttractionItem, i int) {
	if locationData.Properties.Street != "" && locationData.Properties.HouseNumber != "" {
		attractions[i].Address = fmt.Sprintf("%s %s", locationData.Properties.Street, locationData.Properties.HouseNumber)
	} else if locationData.Properties.Street != "" {
		attractions[i].Address = locationData.Properties.Street
	} else if locationData.Properties.HouseNumber != "" {
		attractions[i].Address = locationData.Properties.HouseNumber
	} else {
		attractions[i].Address = ""
	}
}

// processPlanTripJob processes a trip planning job
func (w *Worker) processPlanTripJob(msg *types.PlanTripMessage) error {
	fmt.Printf("📋 Processing plan trip job: %s for trip: %s\n", msg.JobID, msg.TripID)

	// Update job status to running - fetching trip details
	err := w.jobService.UpdateJobProgress(msg.JobID, 5, "Fetching trip details", "fetching_trip")
	if err != nil {
		fmt.Printf("Error updating job progress: %v\n", err)
		return err
	}

	// Fetch trip with destinations and locations via GraphQL
	trip, err := w.supabase.GetTripByID(msg.TripID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch trip: %v", err)
		fmt.Printf("Error fetching trip: %v\n", err)
		if markErr := w.jobService.MarkJobFailed(msg.JobID, errMsg); markErr != nil {
			fmt.Printf("Error marking job as failed: %v\n", markErr)
		}
		return fmt.Errorf("trip not found: %w", err)
	}

	fmt.Printf("✅ Fetched trip: %s (%s to %s) with %d destinations\n",
		trip.Name, trip.StartDate, trip.EndDate, len(trip.Destinations))

	// Update progress - fetching preferences
	err = w.jobService.UpdateJobProgress(msg.JobID, 10, "Fetching itinerary preferences", "fetching_preferences")
	if err != nil {
		fmt.Printf("Error updating job progress: %v\n", err)
	}

	// Fetch preferences
	preferences, err := w.supabase.GetItineraryPreferencesByID(msg.PreferencesID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch preferences: %v", err)
		fmt.Printf("Error fetching preferences: %v\n", err)
		if markErr := w.jobService.MarkJobFailed(msg.JobID, errMsg); markErr != nil {
			fmt.Printf("Error marking job as failed: %v\n", markErr)
		}
		return fmt.Errorf("preferences not found: %w", err)
	}

	fmt.Printf("✅ Fetched preferences: Style=%s, Pace=%s, Budget=%s\n",
		preferences.TravelStyle, preferences.Pace, preferences.BudgetLevel)

	// Update progress - starting AI generation
	err = w.jobService.UpdateJobProgress(msg.JobID, 20, "Starting itinerary generation", "preparing")
	if err != nil {
		fmt.Printf("Error updating job progress: %v\n", err)
	}

	// Update progress
	err = w.jobService.UpdateJobProgress(msg.JobID, 50, "Generating itinerary with AI", "generating")
	if err != nil {
		fmt.Printf("Error updating job progress: %v\n", err)
	}

	// Generate actual itinerary using AI
	itinerary, err := w.aiService.GenerateTripItinerary(trip, preferences)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to generate itinerary with AI: %v", err)
		fmt.Printf("❌ %s\n", errMsg)
		if markErr := w.jobService.MarkJobFailed(msg.JobID, errMsg); markErr != nil {
			fmt.Printf("Error marking job as failed: %v\n", markErr)
		}
		return err
	}

	fmt.Printf("🤖 AI generated itinerary: %d days, %d total activities\n",
		len(itinerary.Itinerary.Days),
		func() int {
			total := 0
			for _, day := range itinerary.Itinerary.Days {
				total += len(day.Activities)
			}
			return total
		}())

	// Update progress - enriching locations
	err = w.jobService.UpdateJobProgress(msg.JobID, 70, "Enriching locations with accurate coordinates", "enriching")
	if err != nil {
		fmt.Printf("Error updating job progress: %v\n", err)
	}

	// Enrich activities with tiered geocoding services
	// Photon (free) → LocationIQ (free tier) → Save without coords
	photonSuccess, locationiqSuccess, totalActivities := w.photonService.EnrichItineraryWithFallback(itinerary, w.locationiqService)
	totalEnriched := photonSuccess + locationiqSuccess

	fmt.Printf("📍 Geocoding results:\n")
	fmt.Printf("   • Photon (free): %d/%d activities\n", photonSuccess, totalActivities)
	if w.locationiqService != nil {
		fmt.Printf("   • LocationIQ (fallback): %d/%d activities\n", locationiqSuccess, totalActivities)
		fmt.Printf("   • Total enriched: %d/%d (%.1f%%)\n", totalEnriched, totalActivities, float64(totalEnriched)/float64(totalActivities)*100)
	} else {
		fmt.Printf("   • Total enriched: %d/%d (%.1f%%)\n", totalEnriched, totalActivities, float64(totalEnriched)/float64(totalActivities)*100)
		fmt.Printf("   • LocationIQ not configured (fallback disabled)\n")
	}

	if totalEnriched == 0 && totalActivities > 0 {
		fmt.Printf("⚠️  Warning: No activities could be enriched with location data\n")
	} else if totalEnriched < totalActivities {
		fmt.Printf("⚠️  Warning: %d activities could not be enriched (will save without coordinates)\n", totalActivities-totalEnriched)
	}

	// Update progress
	err = w.jobService.UpdateJobProgress(msg.JobID, 90, "Saving itinerary to database", "saving")
	if err != nil {
		fmt.Printf("Error updating job progress: %v\n", err)
	}

	// Save activities to database
	savedCount, pointCount, areaCount, err := w.supabase.SaveTripActivities(msg.TripID, trip, itinerary)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to save itinerary: %v", err)
		fmt.Printf("❌ %s\n", errMsg)

		// Mark preferences as failed
		if updateErr := w.supabase.UpdatePreferencesStatus(msg.PreferencesID, "failed", &errMsg); updateErr != nil {
			fmt.Printf("Error updating preferences status: %v\n", updateErr)
		}

		// Mark job as failed
		if markErr := w.jobService.MarkJobFailed(msg.JobID, errMsg); markErr != nil {
			fmt.Printf("Error marking job as failed: %v\n", markErr)
		}
		return err
	}

	fmt.Printf("💾 Saved %d activities (%d point, %d area, %d other)\n",
		savedCount, pointCount, areaCount, savedCount-pointCount-areaCount)

	// Update preferences status to "completed"
	if err := w.supabase.UpdatePreferencesStatus(msg.PreferencesID, "completed", nil); err != nil {
		fmt.Printf("⚠️  Warning: Failed to update preferences status: %v\n", err)
		// Don't fail the job for this - data is already saved
	}

	// Complete job
	if err := w.jobService.CompleteJob(msg.JobID, savedCount); err != nil {
		fmt.Printf("Error completing job: %v\n", err)
		return err
	}

	fmt.Printf("✅ Successfully processed plan trip job for trip: %s\n", msg.TripID)
	return nil
}

// processJobStatusUpdate handles job status updates for logging/monitoring
func (w *Worker) processJobStatusUpdate(msg *types.JobStatusMessage) error {
	// Log status updates
	switch msg.Status {
	case types.JobStatusRunning:
		if msg.Progress != nil {
			fmt.Printf("Job %s progress: %d%% - %s\n", msg.JobID, msg.Progress.Percentage, msg.Progress.Message)
		}
	case types.JobStatusCompleted:
		fmt.Printf("Job %s completed for area %s\n", msg.JobID, msg.AreaKey)
	case types.JobStatusFailed:
		errorMsg := "unknown error"
		if msg.Error != nil {
			errorMsg = *msg.Error
		}
		fmt.Printf("Job %s failed for area %s: %s\n", msg.JobID, msg.AreaKey, errorMsg)
	}

	return nil
}

// Stop gracefully stops the worker
func (w *Worker) Stop() {
	fmt.Printf("Stopping background worker...\n")
	if w.natsService != nil {
		w.natsService.Close()
	}
	fmt.Printf("Background worker stopped\n")
}
