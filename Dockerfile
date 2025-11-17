# Optimized Dockerfile for faster builds
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies (cached layer if go.mod/go.sum unchanged)
RUN go mod download && go mod verify

# Copy only necessary source files
COPY main.go ./

# Build with optimizations for smaller binary and faster build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o pretix-webhook .

# Final stage - use distroless for smaller image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy ca-certificates and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /app/pretix-webhook /usr/local/bin/pretix-webhook

# Expose port
EXPOSE 8080

# Run as non-root user (distroless nonroot)
ENTRYPOINT ["/usr/local/bin/pretix-webhook"]
