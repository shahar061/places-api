# Makefile for Places API

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=places_api
BINARY_UNIX=$(BINARY_NAME)_unix

# Swagger parameters
SWAG_VERSION=v1.16.6
SWAG_CMD=swag

.PHONY: all build clean test deps swagger-install swagger-gen swagger-clean run dev help

# Default target
all: deps swagger-gen build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) -v .

# Build for linux
build-linux:
	@echo "Building for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v .

# Clean build files
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Clean swagger generated files
swagger-clean:
	@echo "Cleaning swagger files..."
	rm -rf docs/

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Install swag tool
swagger-install:
	@echo "Installing swag tool..."
	@which $(SWAG_CMD) > /dev/null 2>&1 || { \
		echo "Installing swag $(SWAG_VERSION)..."; \
		$(GOGET) -u github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION); \
	}
	@echo "Swag tool is ready"

# Generate swagger documentation
swagger-gen: swagger-install
	@echo "Generating swagger documentation..."
	$(SWAG_CMD) init -g main.go --output docs --parseDependency
	@echo "Swagger docs generated in ./docs/"

# Run the server
run: swagger-gen
	@echo "Starting server..."
	$(GOCMD) run main.go server

# Development server with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
dev: swagger-gen
	@echo "Starting development server..."
	@which air > /dev/null 2>&1 && air || { \
		echo "Air not found. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Falling back to regular run..."; \
		$(MAKE) run; \
	}

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run || { \
		echo "golangci-lint not found. Install from https://golangci-lint.run/"; \
		echo "Skipping linting..."; \
	}

# Update dependencies
update:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Show help
help:
	@echo "Available commands:"
	@echo "  all           - Install deps, generate swagger, and build"
	@echo "  build         - Build the binary"
	@echo "  build-linux   - Build for Linux"
	@echo "  clean         - Clean build files"
	@echo "  swagger-clean - Clean swagger generated files"
	@echo "  test          - Run tests"
	@echo "  deps          - Install dependencies"
	@echo "  swagger-install - Install swag tool"
	@echo "  swagger-gen   - Generate swagger documentation"
	@echo "  run           - Generate swagger and run server"
	@echo "  dev           - Development server with auto-reload"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  update        - Update dependencies"
	@echo "  help          - Show this help"
	@echo ""
	@echo "Quick start:"
	@echo "  make swagger-gen  # Generate swagger docs"
	@echo "  make run          # Run the server"
	@echo "  # Visit http://localhost:8080/swagger/index.html for API docs"
