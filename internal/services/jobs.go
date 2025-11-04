package services

import (
	"fmt"
	"places_api/internal/types"
	"time"
)

// JobService handles job management and coordination
type JobService struct {
	supabase *SupabaseService
	nats     *NATSService
}

// NewJobService creates a new job service instance
func NewJobService(supabase *SupabaseService, nats *NATSService) *JobService {
	return &JobService{
		supabase: supabase,
		nats:     nats,
	}
}

// CheckDataFreshness checks if area data is fresh or needs refreshing
func (j *JobService) CheckDataFreshness(areaKey string) (*types.DataFreshness, error) {
	refresh, err := j.supabase.GetAreaRefresh(areaKey)
	if err != nil {
		// No refresh record exists - data is stale
		return &types.DataFreshness{
			IsStale:    true,
			PlaceCount: 0,
		}, nil
	}

	freshness := &types.DataFreshness{
		LastRefreshed: refresh.LastRefreshedAt,
		ExpiresAt:     refresh.DataExpiresAt,
		IsStale:       refresh.NeedsRefresh(),
		PlaceCount:    refresh.PlaceCount,
	}

	return freshness, nil
}

// CreateFetchAttractionsJob creates a new job to fetch attractions for an area
func (j *JobService) CreateFetchAttractionsJob(area *types.Area) (*types.Job, error) {
	// Check if there's already a pending or running job for this area
	existingJob, err := j.supabase.GetLatestJobForArea(area.AreaKey)
	if err == nil && (existingJob.Status == types.JobStatusPending || existingJob.Status == types.JobStatusRunning) {
		// Return existing job instead of creating a new one
		return existingJob, nil
	}

	// Create new job
	job := &types.Job{
		AreaKey:   area.AreaKey,
		JobType:   types.JobTypeFetchAttractions,
		Status:    types.JobStatusPending,
		CreatedAt: time.Now(),
		Progress:  make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
	}

	// Set initial progress
	job.SetProgress(0, "Job created", "initializing")

	// Save to database
	if err := j.supabase.CreateJob(job); err != nil {
		return nil, fmt.Errorf("failed to create job: %v", err)
	}

	// Create area refresh record
	refresh := &types.AreaRefresh{
		AreaKey:            area.AreaKey,
		RefreshRequestedAt: time.Now(),
		Categories:         []string{"attraction", "restaurant", "cafe", "bar", "hotel"},
		PlaceCount:         0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := j.supabase.UpsertAreaRefresh(refresh); err != nil {
		// Log error but don't fail the job creation
		fmt.Printf("Warning: Failed to create area refresh record for area %s: %v\n", area.AreaKey, err)
	} else {
		fmt.Printf("Created area refresh record for area: %s\n", area.AreaKey)
	}

	// Publish to NATS
	msg := &types.FetchAttractionsMessage{
		JobID:       job.ID,
		Area:        *area,
		Categories:  []string{"attraction", "restaurant", "cafe", "bar", "hotel"},
		RequestedAt: time.Now(),
	}

	if err := j.nats.PublishFetchAttractionsJob(msg); err != nil {
		// Update job status to failed
		errorMsg := "Failed to publish job to queue"
		if updateErr := j.supabase.UpdateJobStatus(job.ID, types.JobStatusFailed, nil, &errorMsg); updateErr != nil {
			fmt.Printf("Warning: Failed to update job status to failed: %v\n", updateErr)
		}
		// Note: Job and area refresh are already created in DB, but we return error
		// so caller knows NATS publish failed. The DB records persist.
		fmt.Printf("Warning: Job %s and area refresh created, but NATS publish failed: %v\n", job.ID, err)
		return nil, fmt.Errorf("failed to publish job to NATS (job %s created in DB): %v", job.ID, err)
	}

	fmt.Printf("Created fetch attractions job for area: %s (job ID: %s)\n", area.AreaKey, job.ID)
	return job, nil
}

// GetJobStatus retrieves the current status of a job
func (j *JobService) GetJobStatus(jobID string) (*types.Job, error) {
	return j.supabase.GetJob(jobID)
}

// GetLatestJobForArea retrieves the most recent job for an area
func (j *JobService) GetLatestJobForArea(areaKey string) (*types.Job, error) {
	return j.supabase.GetLatestJobForArea(areaKey)
}

// UpdateJobProgress updates job progress and publishes status update
func (j *JobService) UpdateJobProgress(jobID string, percentage int, message, step string) error {
	// Update progress in database
	progress := map[string]interface{}{
		"percentage": percentage,
		"message":    message,
		"step":       step,
	}

	var status types.JobStatus = types.JobStatusRunning
	if percentage >= 100 {
		status = types.JobStatusCompleted
	}

	if err := j.supabase.UpdateJobStatus(jobID, status, progress, nil); err != nil {
		return fmt.Errorf("failed to update job progress: %v", err)
	}

	// Get updated job for publishing
	job, err := j.supabase.GetJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get updated job: %v", err)
	}

	// Publish status update to NATS
	statusMsg := &types.JobStatusMessage{
		JobID:     jobID,
		AreaKey:   job.AreaKey,
		Status:    status,
		Progress:  job.GetProgress(),
		UpdatedAt: time.Now(),
	}

	if err := j.nats.PublishJobStatus(statusMsg); err != nil {
		// Log error but don't fail the update
		fmt.Printf("Warning: Failed to publish job status update: %v\n", err)
	}

	return nil
}

// MarkJobFailed marks a job as failed with an error message
func (j *JobService) MarkJobFailed(jobID string, errorMessage string) error {
	if err := j.supabase.UpdateJobStatus(jobID, types.JobStatusFailed, nil, &errorMessage); err != nil {
		return fmt.Errorf("failed to mark job as failed: %v", err)
	}

	// Get updated job for publishing
	job, err := j.supabase.GetJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get updated job: %v", err)
	}

	// Publish status update to NATS
	statusMsg := &types.JobStatusMessage{
		JobID:     jobID,
		AreaKey:   job.AreaKey,
		Status:    types.JobStatusFailed,
		Error:     &errorMessage,
		UpdatedAt: time.Now(),
	}

	if err := j.nats.PublishJobStatus(statusMsg); err != nil {
		// Log error but don't fail the update
		fmt.Printf("Warning: Failed to publish job failure status: %v\n", err)
	}

	return nil
}

// CompleteJob marks a job as completed and updates area refresh data
func (j *JobService) CompleteJob(jobID string, placeCount int) error {
	// Get job details
	job, err := j.supabase.GetJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %v", err)
	}

	// Mark job as completed
	progress := map[string]interface{}{
		"percentage": 100,
		"message":    fmt.Sprintf("Completed - fetched %d places", placeCount),
		"step":       "completed",
	}

	if err := j.supabase.UpdateJobStatus(jobID, types.JobStatusCompleted, progress, nil); err != nil {
		return fmt.Errorf("failed to mark job as completed: %v", err)
	}

	// Update area refresh completion
	if err := j.supabase.UpdateAreaRefreshCompleted(job.AreaKey, placeCount); err != nil {
		// Log error but don't fail the completion
		fmt.Printf("Warning: Failed to update area refresh completion: %v\n", err)
	}

	// Publish completion status
	statusMsg := &types.JobStatusMessage{
		JobID:    jobID,
		AreaKey:  job.AreaKey,
		Status:   types.JobStatusCompleted,
		Progress: job.GetProgress(),
		Metadata: map[string]interface{}{
			"place_count": placeCount,
		},
		UpdatedAt: time.Now(),
	}

	if err := j.nats.PublishJobStatus(statusMsg); err != nil {
		// Log error but don't fail the completion
		fmt.Printf("Warning: Failed to publish job completion status: %v\n", err)
	}

	fmt.Printf("Completed job %s for area %s - fetched %d places\n", jobID, job.AreaKey, placeCount)
	return nil
}

// ShouldCreateJob determines if a new job should be created based on data freshness and existing jobs
func (j *JobService) ShouldCreateJob(areaKey string) (bool, *types.Job, error) {
	// Check data freshness
	freshness, err := j.CheckDataFreshness(areaKey)
	if err != nil {
		return false, nil, fmt.Errorf("failed to check data freshness: %v", err)
	}

	// If data is fresh, no need for a job
	if !freshness.IsStale {
		return false, nil, nil
	}

	// Check for existing pending/running jobs
	existingJob, err := j.supabase.GetLatestJobForArea(areaKey)
	if err == nil && (existingJob.Status == types.JobStatusPending || existingJob.Status == types.JobStatusRunning) {
		// Return existing job
		return false, existingJob, nil
	}

	// Should create a new job
	return true, nil, nil
}
