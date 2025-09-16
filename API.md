# Places API v1 - New Design

This document describes the new cache-first API design for the Places API.

## Architecture

The new API is **cache-first**, meaning:

- All public endpoints read from a Supabase cache
- No direct calls to Nominatim/Overpass on the hot path
- Admin endpoints handle background refresh/bootstrap operations

## Public API Endpoints

### 1. Resolve Area

**GET /v1/areas/resolve**

Turn free-text queries like "Rome, Italy" into canonical area keys with geometry.

**Query Parameters:**

- `q` (required): Free-text location query
- `multi` (optional, default=false): Return multiple candidates
- `bootstrap` (optional, default=false): Queue background refresh

**Response (single):**

```json
{
  "area_key": "it_rome",
  "name": "Rome",
  "type": "city",
  "country_code": "IT",
  "admin_level": 8,
  "center": { "lat": 41.9028, "lon": 12.4964 },
  "bbox": {
    "south_lat": 41.7,
    "north_lat": 42.0,
    "west_lon": 12.3,
    "east_lon": 12.7
  },
  "refreshed_at": "2025-09-01T10:12:00Z",
  "refresh_queued": true
}
```

### 2. List Child Areas

**GET /v1/areas/children**

List child areas for drill-down (e.g., cities in a country).

**Query Parameters:**

- `parent` (required): area_key of parent area
- `types` (optional): Comma-separated child types
- `limit` (optional, default=20, max=50)
- `offset` (optional, default=0)

### 3. Top Places for Area

**GET /v1/places/top**

Return ranked places for an area from cache.

**Query Parameters:**

- `area` (required): Canonical area_key
- `cats` (optional): Categories (attraction,restaurant,cafe,bar,hotel)
- `limit` (optional, default=50, max=100)
- `offset` (optional, default=0)
- `lang` (optional): Language preference
- `group_by` (optional): Group by category

### 4. Nearby Places

**GET /v1/places/near**

Fast radial search using PostGIS over cached places.

**Query Parameters:**

- `lat`, `lon` (required): Center coordinates
- `radius` (optional, default=1200m, max=5000m)
- `cats` (optional): Category filter
- `limit` (optional, default=40, max=100)
- `lang` (optional): Language preference

### 5. Search Places

**GET /v1/places/search**

Typeahead search over cached place names.

**Query Parameters:**

- `area` (required): area_key to search within
- `q` (required): Search query
- `cats` (optional): Category filter
- `limit` (optional, default=20, max=50)
- `lang` (optional): Language preference

### 6. Place Details

**GET /v1/places/{id}**

Get detailed information for a specific place.

## Admin API Endpoints

### 1. Bootstrap Area

**POST /v1/admin/areas/bootstrap**

Queue background refresh for an area.

**Request Body:**

```json
{
  "area_key": "it_rome",
  "cats": ["attraction", "restaurant", "cafe", "bar", "hotel"],
  "force": false
}
```

**Response:**

```json
{
  "status": "queued",
  "area_key": "it_rome",
  "job_id": "job_123456"
}
```

### 2. Area Status

**GET /v1/admin/areas/status**

Get refresh status and cache statistics for an area.

**Query Parameters:**

- `area` (required): area_key to check

**Response:**

```json
{
  "area_key": "it_rome",
  "last_refresh_at": "2025-09-12T07:10:00Z",
  "places_count": {
    "attraction": 210,
    "restaurant": 200,
    "cafe": 150
  },
  "stale": false,
  "last_job": {
    "id": "job_12345",
    "status": "ok",
    "duration_ms": 8421
  }
}
```

## Implementation Status

✅ **Completed:**

- API endpoint structure and routing
- Response type definitions
- Mock responses for all endpoints
- Clean removal of old direct API calls

🚧 **TODO - Database Integration:**

- Supabase client setup
- Area cache lookup and storage
- Place cache with PostGIS for spatial queries
- Background job queue for bootstrap/refresh
- Full-text search implementation

🚧 **TODO - Production Features:**

- Authentication for admin endpoints
- Rate limiting
- Caching headers and ETags
- Logging and monitoring
- Error handling improvements
- Input validation

## Testing

Run the application:

```bash
go run main.go
```

The API will be available at `http://localhost:8080` with all endpoints returning mock data until database integration is complete.
