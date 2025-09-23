# Build stage
FROM golang:1.24.6-alpine AS builder

# Install necessary packages
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Install Swagger CLI
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy source code
COPY . .

# Generate Swagger docs
RUN make swagger-gen

# Build the application
RUN make build-linux

# Production stage
FROM alpine:latest

# Install necessary runtime packages
RUN apk add --no-cache ca-certificates tzdata

# Create app user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/places_api_unix ./places_api
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/docs ./docs

# Change ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthcheck || exit 1

# Run the application
CMD ["./places_api", "server"]
