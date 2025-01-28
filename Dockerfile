# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Download dependencies first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/wattwatch ./cmd/api

# Final stage
FROM alpine:3.21

WORKDIR /app

# Add non-root user
RUN adduser -D -g '' appuser

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/wattwatch .
COPY --from=builder /app/migrations ./migrations

# Use non-root user
USER appuser

# Set environment variables
ENV GIN_MODE=release

# Expose the API port
EXPOSE 8080

# Run the application
CMD ["./wattwatch"] 