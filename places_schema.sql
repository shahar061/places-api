-- Places table schema for storing attractions and other places
CREATE TABLE IF NOT EXISTS places (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL, -- attraction, restaurant, cafe, bar, hotel
    short_description TEXT,
    latitude DECIMAL(10, 8) NOT NULL,
    longitude DECIMAL(11, 8) NOT NULL,
    street TEXT,
    area_key TEXT NOT NULL,
    
    -- OSM data
    osm_type TEXT,
    osm_id INTEGER,
    osm_key TEXT,
    osm_value TEXT,
    
    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_places_area_key ON places(area_key);
CREATE INDEX IF NOT EXISTS idx_places_type ON places(type);
CREATE INDEX IF NOT EXISTS idx_places_location ON places(latitude, longitude);
CREATE INDEX IF NOT EXISTS idx_places_osm_id ON places(osm_id);
CREATE INDEX IF NOT EXISTS idx_places_updated_at ON places(updated_at);

-- Composite index for area + type queries
CREATE INDEX IF NOT EXISTS idx_places_area_type ON places(area_key, type);

-- Function to automatically update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at on row updates
CREATE TRIGGER update_places_updated_at 
    BEFORE UPDATE ON places 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Enable Row Level Security (RLS) if needed
-- ALTER TABLE places ENABLE ROW LEVEL SECURITY;

-- Example RLS policies (uncomment if you want to enable RLS)
-- CREATE POLICY "Allow all operations for authenticated users" ON places
--     FOR ALL USING (auth.role() = 'authenticated');

-- CREATE POLICY "Allow read access for anonymous users" ON places
--     FOR SELECT USING (true);
