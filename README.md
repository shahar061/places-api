# Places API

A modern, cache-first REST API for location search and places discovery. Built with Go, Gin, and designed for high performance with comprehensive Swagger documentation.

## 🏗️ Architecture

**Cache-First Design:**

- All public endpoints read from Supabase cache (never hit external APIs directly)
- Background workers handle data refresh from Nominatim/Overpass
- Admin endpoints manage cache warming and job queuing
- PostGIS integration for fast spatial queries

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- Make (for development commands)

### Installation & Setup

```bash
# Clone and install dependencies
git clone https://github.com/shahar061/places_api.git
cd places_api
make deps

# Generate Swagger documentation
make swagger-gen

# Run the server
make run
```

### Access Points

- **API Server:** http://localhost:8080
- **Swagger UI:** http://localhost:8080/swagger/index.html
- **Health Check:** http://localhost:8080/healthcheck

## 📚 API Documentation

### Public API Endpoints

**Areas (Geocoding & Hierarchy)**

- `GET /v1/areas/resolve` - Convert "Rome, Lazio, Italy" → canonical area_key
- `GET /v1/areas/children` - List child areas (cities in country, etc.)

**Places (Search & Discovery)**

- `GET /v1/places/top` - Top-ranked places for an area (from cache)
- `GET /v1/places/near` - Nearby places via PostGIS radial search
- `GET /v1/places/search` - Typeahead search within cached places
- `GET /v1/places/{id}` - Detailed place information

**Admin API (Background Operations)**

- `POST /v1/admin/areas/bootstrap` - Queue area cache refresh
- `GET /v1/admin/areas/status` - Cache status and job information

**System**

- `GET /healthcheck` - Service health status

### Interactive Documentation

Visit http://localhost:8080/swagger/index.html for full interactive API documentation with:

- Complete parameter descriptions
- Request/response schemas
- Live testing interface
- Type definitions

## 🛠️ Development

### Make Commands

```bash
# Development
make run          # Generate swagger + run server
make dev          # Development server with auto-reload (requires air)
make build        # Build binary
make test         # Run tests

# Swagger Documentation
make swagger-gen     # Generate Swagger docs
make swagger-clean   # Clean generated docs
make swagger-install # Install swag tool

# Code Quality
make fmt          # Format code
make lint         # Run linter
make clean        # Clean build files

# Dependencies
make deps         # Install/update dependencies
make update       # Update all dependencies

# Help
make help         # Show all available commands
```

### Project Structure

```
places_api/
├── cmd/                    # CLI commands (Cobra)
├── configs/               # Configuration files
├── docs/                  # Generated Swagger documentation
├── internal/
│   ├── ai/               # AI planner service
│   ├── config/           # Configuration management
│   ├── handlers/         # HTTP request handlers
│   ├── server/           # HTTP server setup
│   ├── services/         # External service integrations
│   ├── types/            # Shared type definitions
│   └── utils/            # Utility functions
├── .github/workflows/     # CI/CD workflows
├── main.go               # Application entry point
├── Makefile             # Development automation
├── Dockerfile            # Container configuration
└── README.md            # This file
```

## 🎯 Key Features

### Cache-First Performance

- **Fast Response Times:** All reads from local cache
- **High Availability:** No dependency on external APIs for reads
- **Scalable:** Background workers handle data refresh

### Modern Go Architecture

- **Clean Separation:** Types, handlers, and business logic decoupled
- **Type Safety:** Comprehensive type definitions in dedicated package
- **Standard Libraries:** Gin, Cobra, Viper for proven reliability

### Comprehensive Documentation

- **Interactive Swagger UI** with live testing
- **Auto-Generated Docs** from Go code annotations
- **Type-Safe Schemas** for all requests/responses

### Developer Experience

- **One-Command Setup:** `make run` gets you started
- **Hot Reloading:** `make dev` for rapid development
- **Comprehensive Makefile** for all common tasks

## 📊 API Response Examples

### Resolve Area

```bash
curl "http://localhost:8080/v1/areas/resolve?q=Rome"
```

```json
{
  "area_key": "it_rome",
  "name": "Rome",
  "type": "city",
  "country_code": "IT",
  "center": { "lat": 41.9028, "lon": 12.4964 },
  "bbox": {
    "south_lat": 41.7,
    "north_lat": 42.0,
    "west_lon": 12.3,
    "east_lon": 12.7
  }
}
```

### Top Places

```bash
curl "http://localhost:8080/v1/places/top?area=it_rome"
```

```json
{
  "area_key": "it_rome",
  "results": [
    {
      "id": "9f9a_c1",
      "name": "Colosseum",
      "category": "attraction",
      "lat": 41.8902,
      "lon": 12.4922,
      "popularity": 98.4
    }
  ]
}
```

## 🏃‍♂️ Running in Production

### Build for Production

```bash
make build-linux    # Cross-compile for Linux
# Or
make build         # Build for current platform
```

### Configuration

The application uses `configs/config.yaml` for configuration. Key settings:

- Server host/port
- Database connection (Supabase)
- Cache settings
- Logging levels

### Docker Support

_Coming soon - Docker support for containerized deployment_

## 🔧 Configuration

### Environment Variables

- `CONFIG_FILE` - Path to config file (default: `./configs/config.yaml`)

### Config File Structure

```yaml
server:
  host: localhost
  port: 8080
# Additional configuration options...
```

## 🚧 Implementation Status

### ✅ Completed

- **API Architecture:** Complete cache-first design
- **Endpoint Structure:** All 8 endpoints implemented with mock data
- **Type System:** Comprehensive type definitions in dedicated package
- **Swagger Integration:** Full documentation with interactive UI
- **Development Tools:** Complete Makefile with all necessary commands
- **Code Quality:** Clean architecture, linting, formatting

### 🚧 In Progress / TODO

- **Database Integration:** Supabase client and PostGIS queries
- **Background Jobs:** Area bootstrap and refresh workers
- **Authentication:** Admin endpoint protection
- **Caching:** Redis/memory caching layer
- **Monitoring:** Logging, metrics, and health checks
- **Docker:** Containerization for deployment

## 📝 Development Workflow

1. **Start Development:**

   ```bash
   make dev    # Auto-reload server
   ```

2. **Make Changes:**

   - Add/modify endpoints in `internal/handlers/`
   - Update types in `internal/types/`
   - Add Swagger annotations

3. **Update Documentation:**

   ```bash
   make swagger-gen    # Regenerate docs
   ```

4. **Test Changes:**

   - Visit http://localhost:8080/swagger/index.html
   - Test endpoints interactively

5. **Quality Check:**
   ```bash
   make fmt lint test    # Format, lint, test
   ```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch
3. Add appropriate tests and documentation
4. Run `make fmt lint test` to ensure quality
5. Submit a pull request

## 📋 API Design Principles

- **Cache-First:** Never hit external APIs on hot path
- **RESTful:** Standard HTTP methods and status codes
- **Consistent:** Uniform response formats across endpoints
- **Documented:** Comprehensive Swagger documentation
- **Type-Safe:** Strong typing throughout the application
- **Performant:** Optimized for high-traffic scenarios

## 📚 Additional Documentation

- [API.md](./API.md) - Detailed API specification and design
- [SWAGGER.md](./SWAGGER.md) - Swagger setup and usage guide
- [DEPLOYMENT.md](./DEPLOYMENT.md) - CI/CD and deployment instructions
- [AI_PLANNER.md](./AI_PLANNER.md) - AI travel planning service guide
- [SUPABASE_SETUP.md](./SUPABASE_SETUP.md) - Database setup instructions

## 🐛 Troubleshooting

### Server Won't Start

1. Check if port 8080 is available
2. Verify config file exists: `configs/config.yaml`
3. Run `make swagger-gen` to ensure docs are generated

### Swagger UI Not Loading

1. Generate docs: `make swagger-gen`
2. Check `/swagger/doc.json` endpoint
3. Verify imports in `internal/handlers/routes.go`

### Build Errors

1. Clean and rebuild: `make clean && make build`
2. Update dependencies: `make update`
3. Check Go version (requires 1.24+)

---

**Places API** - Built with ❤️ using Go, Gin, and modern development practices.
