# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /tulis-api ./cmd/api

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install runtime dependencies (ca-certificates for HTTPS, tzdata for time)
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Copy binary from builder
COPY --from=builder /tulis-api /app/tulis-api

# Copy .env file (can be overridden at runtime via volume mount)
COPY .env.example /app/.env.example
COPY .env /app/.env

# Create uploads directory
RUN mkdir -p /app/uploads && chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
# HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
#     CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/tulis-api"]
