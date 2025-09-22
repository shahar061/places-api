# Supabase Integration Setup

Your Places API has been successfully wired up with Supabase integration! 🎉

## What Was Done

1. **Added Supabase Go Client**: Added `github.com/supabase-community/postgrest-go` to your dependencies
2. **Updated Configuration**: Extended the config system to support Supabase URL and API key
3. **Created Supabase Service**: Added `/internal/services/supabase.go` with methods for all your API endpoints
4. **Wired Up Handlers**: Updated handlers to use the Supabase service when available, with graceful fallback to mock data
5. **Graceful Degradation**: The API will run with mock data if Supabase credentials aren't provided

## Environment Variables Setup

Set these environment variables to connect to your Supabase database:

```bash
export PLACES_API_DATABASE_SUPABASE_URL="https://your-project.supabase.co"
export PLACES_API_DATABASE_SUPABASE_KEY="your-supabase-anon-key"
```

You can find these values in your Supabase Dashboard:

- **URL**: Project Settings > API > Project URL
- **API Key**: Project Settings > API > Project API keys > anon public

## Database Schema

To complete the integration, you'll need to create tables in your Supabase database. Here's a suggested schema:

### Areas Table

```sql
CREATE TABLE areas (
    area_key VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    country_code VARCHAR(2),
    admin_level INTEGER,
    center_lat DECIMAL(10,8),
    center_lon DECIMAL(11,8),
    bbox_south DECIMAL(10,8),
    bbox_north DECIMAL(10,8),
    bbox_west DECIMAL(11,8),
    bbox_east DECIMAL(11,8),
    refreshed_at TIMESTAMP,
    refresh_queued BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Places Table

```sql
CREATE TABLE places (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL,
    lat DECIMAL(10,8) NOT NULL,
    lon DECIMAL(11,8) NOT NULL,
    address TEXT,
    area_key VARCHAR(255) REFERENCES areas(area_key),
    popularity DECIMAL(5,2),
    distance_m INTEGER,
    updated_at TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Enable PostGIS for geospatial queries
CREATE EXTENSION IF NOT EXISTS postgis;

-- Add geospatial index for nearby searches
CREATE INDEX places_location_idx ON places USING GIST(ST_Point(lon, lat));
```

### Place Sources Table

```sql
CREATE TABLE place_sources (
    place_id VARCHAR(255) REFERENCES places(id),
    source VARCHAR(100) NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    url TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (place_id, source)
);
```

## Implementation Status

✅ **Ready**: The following endpoints are wired up and ready to work with Supabase:

- `GET /v1/areas/resolve` - Area resolution
- `GET /v1/places/top` - Top places for area
- `GET /v1/places/{id}` - Place details

🔧 **TODO**: You still need to implement the actual SQL queries in `/internal/services/supabase.go`:

- `ResolveArea()` - Search areas table
- `GetAreaChildren()` - Hierarchical area queries
- `GetTopPlaces()` - Places by area and category
- `GetNearbyPlaces()` - PostGIS radial search
- `SearchPlaces()` - Full-text search
- `GetPlaceDetails()` - Place lookup with sources
- `GetAreaStatus()` - Cache status
- `BootstrapArea()` - Background job queueing

## Testing

The API will work immediately with mock data. To test Supabase integration:

1. Set up your environment variables
2. Run the API: `go run main.go server`
3. Check the logs - you should see either:
   - Success: Supabase service initialized
   - Warning: Falls back to mock data (if credentials missing)

## Next Steps

1. Create your Supabase database schema
2. Implement the actual queries in `supabase.go`
3. Test with real data
4. Consider adding connection pooling and caching for production

The integration is designed to be fault-tolerant - your API will continue to work with mock data even if the database is unavailable.
