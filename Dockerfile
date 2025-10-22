# Multi-stage build for market-data-simulator-go
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /build

# Install git and ca-certificates (needed for dependency fetching)
RUN apk add --no-cache git ca-certificates

# Copy market-data-adapter-go dependency first (from parent context)
COPY market-data-adapter-go/ ./market-data-adapter-go/

# Copy market-data-simulator-go files
COPY market-data-simulator-go/go.mod market-data-simulator-go/go.sum ./market-data-simulator-go/

# Set working directory to market-data-simulator-go
WORKDIR /build/market-data-simulator-go

# Download dependencies (now can find ../market-data-adapter-go)
RUN go mod download

# Copy source code
COPY market-data-simulator-go/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o market-data-simulator ./cmd/server

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS connections and wget for health checks
RUN apk --no-cache add ca-certificates wget

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /build/market-data-simulator-go/market-data-simulator /app/market-data-simulator

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 50051

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Run the application
CMD ["./market-data-simulator"]