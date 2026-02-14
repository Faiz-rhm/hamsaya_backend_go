# How to Run the Hamsaya Backend

Step-by-step guide to run the API server locally.

---

## Prerequisites

| Requirement | Purpose |
|-------------|---------|
| **Go 1.21+** | [Download](https://go.dev/dl/) |
| **Docker & Docker Compose** | PostgreSQL, Redis, MinIO ([Download](https://docs.docker.com/get-docker/)) |
| **Make** (optional) | Shorthand for commands below |

**Optional (for image processing):** On macOS with Homebrew, install for WebP support:

```bash
brew install pkg-config webp
```

---

## Quick Start (5 steps)

All commands are run from the backend project root: `hamsaya_backend_go/`.

### 1. Environment

```bash
cp .env.example .env
```

**If you use Docker for Postgres/Redis/MinIO** (recommended), set the DB port in `.env`:

```bash
# In .env – Docker exposes Postgres on 5433 on the host
DB_PORT=5433
```

Leave other defaults as-is for local development.

### 2. Dependencies

```bash
go mod download
# or: make deps
```

### 3. Start infrastructure

```bash
docker-compose up -d postgres redis minio
# or: make docker-up
```

Wait ~10–15 seconds, then check containers:

```bash
docker ps
# hamsaya-postgres, hamsaya-redis, hamsaya-minio should be "Up" and (healthy)
```

### 4. Migrations

```bash
go run cmd/migrate/main.go up
# or: make migrate-up
```

You should see: `All migrations applied successfully`.

### 5. Run the server

```bash
go run cmd/server/main.go
# or: make run
```

Server listens at **http://localhost:8080**.

**Verify:**

```bash
curl http://localhost:8080/health
# {"success":true,"message":"OK",...}
```

```bash
curl http://localhost:8080/health/ready
# DB and Redis should be "healthy"
```

---

## Command reference

| Task | Command |
|------|---------|
| Start infra only | `docker-compose up -d postgres redis minio` |
| Stop infra | `docker-compose down` |
| Migrate up | `go run cmd/migrate/main.go up` or `make migrate-up` |
| Migrate down | `make migrate-down` |
| Run API | `go run cmd/server/main.go` or `make run` |
| Build binary | `go build -o bin/server cmd/server/main.go` or `make build` |
| Run with hot reload | `make install-air` (once), then `make dev` |

---

## Environment summary

| Variable | Local default | Notes |
|----------|----------------|-------|
| `SERVER_PORT` | 8080 | API port |
| `DB_HOST` | localhost | Use `postgres` if API runs in Docker |
| `DB_PORT` | **5433** | Use 5433 when Postgres is from Docker (host mapping) |
| `DB_NAME` | hamsaya | |
| `DB_USER` / `DB_PASSWORD` | postgres / postgres | Match `docker-compose.yml` |
| `REDIS_HOST` | localhost | Use `redis` if API runs in Docker |
| `JWT_SECRET` | (from .env.example) | **Change in production** |
| `STORAGE_*` | MinIO defaults | Matches `docker-compose` |

---

## Troubleshooting

### "role \"postgres\" does not exist" or connection refused

- If using **Docker** for Postgres, set `DB_PORT=5433` in `.env` (see `docker-compose.yml` port mapping).
- If using a **local Postgres** (no Docker), use `DB_PORT=5432` and ensure the `postgres` role exists.

### "pkg-config" or "libwebp" not found when building

Image processing (e.g. WebP) needs system libraries. On macOS:

```bash
brew install pkg-config webp
```

### Migrations fail (e.g. "invalid geometry" or index errors)

- Ensure all containers are **healthy**: `docker ps`.
- For a clean DB, remove the volume and re-run migrations:
  ```bash
  docker-compose down -v
  docker-compose up -d postgres redis minio
  # wait for healthy
  go run cmd/migrate/main.go up
  ```

### Port 8080 already in use

- Stop the process using 8080, or set another port in `.env`: `SERVER_PORT=8081`.

### Rate limit (e.g. on login)

- Auth endpoints are rate-limited. Wait a few minutes or clear Redis:
  ```bash
  docker exec hamsaya-redis redis-cli FLUSHDB
  ```

---

## Running the full stack in Docker

To run the API inside Docker as well (all services in containers):

```bash
docker-compose up -d
```

API is still at http://localhost:8080. Logs:

```bash
docker-compose logs -f api
```

For day-to-day development, running the API **on the host** (`go run cmd/server/main.go`) with only Postgres/Redis/MinIO in Docker is usually easier (faster rebuilds, easier debugging).

---

## Next steps

- **API docs (Swagger):** http://localhost:8080/swagger/index.html  
- **Health checks:** [HEALTH_CHECKS.md](../HEALTH_CHECKS.md)  
- **Deployment:** [DEPLOYMENT.md](../DEPLOYMENT.md)
