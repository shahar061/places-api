-- Places API Database Schema
-- Run this SQL in your Supabase SQL editor to set up the required tables

-- Jobs table for tracking background tasks
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    area_key VARCHAR NOT NULL,
    job_type VARCHAR NOT NULL,
    status VARCHAR NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    progress JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}'
);

-- Area refresh tracking
CREATE TABLE IF NOT EXISTS area_refreshes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    area_key VARCHAR NOT NULL UNIQUE,
    last_refreshed_at TIMESTAMP WITH TIME ZONE,
    refresh_requested_at TIMESTAMP WITH TIME ZONE,
    data_expires_at TIMESTAMP WITH TIME ZONE,
    categories TEXT[] DEFAULT ARRAY['attraction','restaurant','cafe','bar','hotel'],
    place_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_jobs_area_key ON jobs(area_key);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_area_refreshes_expires ON area_refreshes(data_expires_at);

-- Comments for documentation
COMMENT ON TABLE jobs IS 'Background jobs for processing area data';
COMMENT ON TABLE area_refreshes IS 'Tracks when area data was last refreshed and when it expires';

COMMENT ON COLUMN jobs.job_type IS 'Type of job: fetch_attractions, etc.';
COMMENT ON COLUMN jobs.status IS 'Job status: pending, running, completed, failed';
COMMENT ON COLUMN jobs.progress IS 'JSON object with percentage, message, step, etc.';
COMMENT ON COLUMN jobs.metadata IS 'Additional job-specific data';

COMMENT ON COLUMN area_refreshes.area_key IS 'Canonical area identifier';
COMMENT ON COLUMN area_refreshes.data_expires_at IS 'When the cached data expires (30 days TTL)';
COMMENT ON COLUMN area_refreshes.categories IS 'Categories that were fetched for this area';
COMMENT ON COLUMN area_refreshes.place_count IS 'Number of places fetched in last refresh';
