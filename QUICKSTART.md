# Quick Start Guide

Get the Hamsaya Backend API running in minutes.

## Prerequisites

- **Go 1.21+** - [Install Go](https://go.dev/dl/)
- **Docker & Docker Compose** - [Install Docker](https://docs.docker.com/get-docker/)
- **Make** (optional, but recommended)

## Quick Setup (5 minutes)

### 1. Install Dependencies

```bash
make deps
# or
go mod download
```

### 2. Start Infrastructure Services

Start PostgreSQL, Redis, and MinIO using Docker:

```bash
docker-compose up -d postgres redis minio
# or
make docker-up
```

Wait ~10 seconds for services to be ready.

### 3. Run Database Migrations

```bash
make migrate-up
```

**Note:** If you encounter a `CREATE INDEX CONCURRENTLY` error, it's safe to ignore - the migration will continue on the next run.

### 4. Configure Environment (Optional)

Create a `.env` file if you need custom settings:

```bash
cp .env.example .env  # Edit as needed
```

**Minimum required environment variables** (defaults work for local dev):
- `DB_HOST=localhost` (or `postgres` if using Docker)
- `DB_PORT=5433` (5432 in Docker)
- `DB_NAME=hamsaya`
- `DB_USER=postgres`
- `DB_PASSWORD=postgres`
- `REDIS_HOST=localhost` (or `redis` if using Docker)
- `REDIS_PORT=6379`
- `JWT_SECRET=dev-secret-change-in-production`

### 5. Run the Server

```bash
make run
# or
go run cmd/server/main.go
```

The API will be available at **http://localhost:8080**

## Verify It's Working

```bash
# Health check
curl http://localhost:8080/health

# API docs
open http://localhost:8080/swagger/index.html
```

## Images not loading? (MinIO)

Post images are served from **MinIO** on port **9000**. If image URLs like `http://...:9000/hamsaya-uploads/post/xxx.webp` don't load, start MinIO (and optionally Postgres/Redis if you use Docker for them):

```bash
docker-compose up -d postgres redis minio
```

Then ensure your backend `.env` has a `CDN_URL` your app can reach:

- **iOS Simulator:** `CDN_URL=http://127.0.0.1:9000`
- **Physical device (same Wi‑Fi as Mac):** `CDN_URL=http://YOUR_MAC_IP:9000` (e.g. `http://192.168.100.17:9000`)

Restart the API after changing `.env`.

## Common Commands

```bash
# Development
make run              # Run the server
make dev              # Run with hot reload (requires air)
make test             # Run tests
make lint             # Run linter

# Database
make migrate-up       # Apply migrations
make migrate-down     # Rollback last migration
make migrate-status   # Check migration status
make seed             # Seed sample data
make seed-demo        # Seed comprehensive demo data

# Docker
make docker-up        # Start all services
make docker-down      # Stop all services
make docker-logs      # View logs

# Cleanup
make clean            # Remove build artifacts
```

## Troubleshooting

### Port Already in Use

If port 8080 is taken, set a different port:
```bash
SERVER_PORT=8081 make run
```

### Database Connection Failed

1. Check Docker containers are running:
   ```bash
   docker ps
   ```

2. Verify database is accessible:
   ```bash
   docker exec -it hamsaya-postgres psql -U postgres -d hamsaya
   ```

### Migration Errors

If migrations fail with `CREATE INDEX CONCURRENTLY` error:
- This is expected for concurrent index creation
- The migration will skip and continue on next run
- Most migrations will have already applied successfully

### Redis Connection Failed

Check Redis is running:
```bash
docker exec -it hamsaya-redis redis-cli ping
# Should return: PONG
```

## Next Steps

- Read the full [README.md](./README.md) for detailed documentation
- Check [API_DOCUMENTATION.md](./API_DOCUMENTATION.md) for API endpoints
- Review [DEPLOYMENT.md](./DEPLOYMENT.md) for production setup

## Stop Everything

```bash
# Stop the server (Ctrl+C)
# Stop Docker services
make docker-down
# or
docker-compose down
```

---

**Need help?** Check the main [README.md](./README.md) or review the logs.

