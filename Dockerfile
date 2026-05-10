# Build stage
FROM golang:1.25.10-alpine AS builder

# Install dependencies (libwebp for go-webp, gcc/musl-dev for cgo)
RUN apk add --no-cache git make gcc musl-dev libwebp-dev

# Set working directory
WORKDIR /app

# Copy source and vendored modules (build offline, no go mod download)
COPY . .

# Build binary (CGO required for go-webp). Uses -mod=mod so deps are resolved at build time.
RUN CGO_ENABLED=1 GOOS=linux go build -mod=mod -a -o main ./cmd/server

# Final stage
FROM alpine:latest

# Install runtime deps:
#   ca-certificates, tzdata — TLS + timezone
#   libwebp                 — runtime for go-webp
#   postgresql-client       — pg_dump used by the backup job
#   gnupg                   — symmetric encryption of dumps before upload
RUN apk --no-cache add ca-certificates tzdata libwebp postgresql-client gnupg

# Set timezone
ENV TZ=UTC

# Create app user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

WORKDIR /home/app

# Copy binary from builder
COPY --from=builder --chown=app:app /app/main .
COPY --from=builder --chown=app:app /app/migrations ./migrations

# Backup target directory. Created+chowned BEFORE the named docker volume
# mounts so the volume inherits app:app ownership on first mount (Docker
# named-volume init copies the source path's permissions). Without this
# step the dir would be root-owned and the non-root app user could not
# write encrypted dumps to it.
RUN mkdir -p /var/lib/hamsaya/backups && \
    chown -R app:app /var/lib/hamsaya && \
    chmod 750 /var/lib/hamsaya/backups

# gpg writes random pool state to $HOME/.gnupg; create it under the app
# user's HOME so symmetric encryption doesn't fail at runtime.
RUN mkdir -p /home/app/.gnupg && \
    chown -R app:app /home/app/.gnupg && \
    chmod 700 /home/app/.gnupg

# Switch to non-root user
USER app

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
CMD ["./main"]
