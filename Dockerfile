# Osprey Production Dockerfile
# Multi-stage build for minimal image size

# Build stage
FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}" \
    -trimpath \
    -o osprey \
    ./cmd/osprey

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates wget

# Copy binary from builder
COPY --from=builder /build/osprey /app/osprey

# Create non-root user
RUN addgroup -g 1000 osprey && \
    adduser -u 1000 -G osprey -s /bin/sh -D osprey && \
    mkdir -p /app/data && \
    chown -R osprey:osprey /app

USER osprey

EXPOSE 8080

VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q -O - http://localhost:8080/health >/dev/null || exit 1

ENTRYPOINT ["/app/osprey"]
