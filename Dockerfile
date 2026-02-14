# Build stage
FROM golang:1.24-alpine AS builder

# Install dependencies (libwebp for go-webp, gcc/musl-dev for cgo)
RUN apk add --no-cache git make gcc musl-dev libwebp-dev

# Set working directory
WORKDIR /app

# Copy source and vendored modules (build offline, no go mod download)
COPY . .

# Build binary using vendor (CGO required for go-webp)
RUN CGO_ENABLED=1 GOOS=linux go build -mod=vendor -a -o main ./cmd/server

# Final stage
FROM alpine:latest

# Install ca-certificates and libwebp (runtime for go-webp)
RUN apk --no-cache add ca-certificates tzdata libwebp

# Set timezone
ENV TZ=UTC

# Create app user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

WORKDIR /home/app

# Copy binary from builder
COPY --from=builder --chown=app:app /app/main .
COPY --from=builder --chown=app:app /app/migrations ./migrations

# Switch to non-root user
USER app

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
CMD ["./main"]
