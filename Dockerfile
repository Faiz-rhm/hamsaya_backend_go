# Build stage
FROM golang:1.25.12-alpine AS builder

# Install dependencies (libwebp for go-webp, gcc/musl-dev for cgo)
RUN apk add --no-cache git make gcc musl-dev libwebp-dev

# Set working directory
WORKDIR /app

# Copy source and vendored modules (build offline, no go mod download)
COPY . .

# Server / seed-demo / seed-master / seed-admin transitively import
# pkg/storage which needs go-webp (CGO). migrate / db-reset /
# backfill-notifications are pure Go.
RUN CGO_ENABLED=1 GOOS=linux go build -mod=mod -a -o /out/main             ./cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -mod=mod -a -o /out/seed-master      ./cmd/seed-master
RUN CGO_ENABLED=1 GOOS=linux go build -mod=mod -a -o /out/seed-demo        ./cmd/seed-demo
RUN CGO_ENABLED=1 GOOS=linux go build -mod=mod -a -o /out/seed-admin       ./cmd/seed-admin
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -a -o /out/migrate          ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -a -o /out/db-reset         ./cmd/db-reset
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -a -o /out/backfill-notifications ./cmd/backfill-notifications

# Final stage
FROM alpine:latest

# Install runtime deps:
#   ca-certificates, tzdata — TLS + timezone
#   libwebp                 — runtime for go-webp
#   postgresql-client       — pg_dump + psql for backups + ops tooling
#   gnupg                   — symmetric encryption of dumps before upload
#   bash, make              — let operators run helper commands
RUN apk --no-cache add ca-certificates tzdata libwebp postgresql-client gnupg bash make

# Set timezone
ENV TZ=UTC

# Create app user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app -s /bin/bash

# Use /app as the working directory so it matches dev expectations.
WORKDIR /app

# Copy binaries from builder.
COPY --from=builder --chown=app:app /out/main                    ./main
COPY --from=builder --chown=app:app /out/migrate                 ./migrate
COPY --from=builder --chown=app:app /out/seed-master             ./seed-master
COPY --from=builder --chown=app:app /out/seed-demo               ./seed-demo
COPY --from=builder --chown=app:app /out/seed-admin              ./seed-admin
COPY --from=builder --chown=app:app /out/db-reset                ./db-reset
COPY --from=builder --chown=app:app /out/backfill-notifications  ./backfill-notifications

# Migrations + entrypoint + Makefiles.
#   Makefile (== Makefile.master) — production-safe, idempotent ops.
#                                   Default `make` target list.
#   Makefile.demo                  — destructive / demo-only ops.
#                                   Invoke with `make -f Makefile.demo <target>`.
COPY --from=builder --chown=app:app /app/migrations       ./migrations
COPY --from=builder --chown=app:app /app/scripts          ./scripts
COPY --from=builder --chown=app:app /app/Makefile.master  ./Makefile
COPY --from=builder --chown=app:app /app/Makefile.master  ./Makefile.master
COPY --from=builder --chown=app:app /app/Makefile.demo    ./Makefile.demo
RUN chmod +x ./scripts/entrypoint.sh

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

# Run migrations then start server
CMD ["./scripts/entrypoint.sh"]
