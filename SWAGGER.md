# Swagger Documentation

This document explains how to generate and use Swagger documentation for the Places API.

## Quick Start

1. **Generate Swagger docs and run server:**

   ```bash
   make run
   # OR manually:
   make swagger-gen
   go run main.go server
   ```

2. **Access Swagger UI:**

   ```
   http://localhost:8080/swagger/index.html
   ```

3. **Access raw Swagger JSON:**
   ```
   http://localhost:8080/swagger/doc.json
   ```

## Available Make Commands

### Swagger-related commands:

- `make swagger-install` - Install the swag tool
- `make swagger-gen` - Generate Swagger documentation
- `make swagger-clean` - Clean generated swagger files

### General commands:

- `make run` - Generate swagger docs and run server
- `make build` - Build the binary
- `make test` - Run tests
- `make help` - Show all available commands

## Generated Files

When you run `make swagger-gen`, the following files are created in the `docs/` directory:

- `docs/docs.go` - Go package with embedded swagger data
- `docs/swagger.json` - Swagger specification in JSON format
- `docs/swagger.yaml` - Swagger specification in YAML format

## API Documentation Structure

The Swagger documentation includes:

### Public API Endpoints

- **Areas**
  - `GET /v1/areas/resolve` - Resolve area from text query
  - `GET /v1/areas/children` - List child areas
- **Places**
  - `GET /v1/places/top` - Get top places for area
  - `GET /v1/places/near` - Find nearby places
  - `GET /v1/places/search` - Search places by name
  - `GET /v1/places/{id}` - Get place details

### Admin API Endpoints

- `POST /v1/admin/areas/bootstrap` - Bootstrap area cache
- `GET /v1/admin/areas/status` - Get area cache status

### System Endpoints

- `GET /healthcheck` - Health check

## Swagger Annotations

The API uses the following Swagger annotations in the handler functions:

- `@Summary` - Brief description
- `@Description` - Detailed description
- `@Tags` - Groups endpoints in UI
- `@Accept` - Request content type
- `@Produce` - Response content type
- `@Param` - Parameter descriptions
- `@Success` - Success response schema
- `@Failure` - Error response schema
- `@Router` - HTTP method and path

## Response Types

All response types are defined in `internal/types/types.go`:

- `Area` - Geographic area information
- `Place` - Basic place information
- `PlaceDetail` - Detailed place information
- `BootstrapRequest/Response` - Admin API types
- `AreaStatus` - Cache status information

## Troubleshooting

### Swagger UI not accessible

1. Make sure you've run `make swagger-gen` to generate docs
2. Check that the server is running: `curl http://localhost:8080/healthcheck`
3. Verify swagger endpoint: `curl http://localhost:8080/swagger/doc.json`

### Build errors

1. Clean and regenerate: `make swagger-clean && make swagger-gen`
2. Check Go imports in `internal/handlers/routes.go`
3. Ensure swag tool is installed: `make swagger-install`

### Missing type definitions

The swag tool automatically generates type definitions from Go structs.
If types are missing, check that they're properly referenced in handler annotations.

## Development Workflow

1. Add new endpoints with swagger annotations
2. Run `make swagger-gen` to update documentation
3. Test API using Swagger UI
4. Commit both code and generated docs

## Production Notes

- The swagger endpoint (`/swagger/*`) should be disabled in production
- Use environment variables to control swagger availability
- Consider adding authentication to swagger endpoints if needed

## Next Steps

- Add authentication examples to swagger docs
- Include response examples for each endpoint
- Add more detailed parameter validation
- Consider adding API versioning information
