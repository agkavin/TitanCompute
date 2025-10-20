# Multi-stage build for Go coordinator
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make protobuf-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o coordinator ./cmd/main.go

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1001 -S titan && \
    adduser -S -D -H -u 1001 -h /app -s /sbin/nologin -G titan -g titan titan

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder --chown=titan:titan /app/coordinator ./coordinator

# Create config directory
RUN mkdir -p /app/config && chown -R titan:titan /app

USER titan

EXPOSE 50051 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD nc -z localhost 50051 || exit 1

CMD ["./coordinator"]
