# Local development – URLs & links

When you run the backend with Docker (`docker-compose up -d postgres redis minio`), these are the URLs and ports.

---

## API & docs

| Service | URL | Description |
|--------|-----|-------------|
| **API base** | http://localhost:8080 | Backend API root |
| **Health** | http://localhost:8080/health | Basic health check |
| **Health ready** | http://localhost:8080/health/ready | DB + Redis status |
| **Swagger UI** | http://localhost:8080/swagger/index.html | Interactive API docs |
| **Metrics** | http://localhost:8080/health/metrics | Prometheus-style metrics |
| **Ping** | http://localhost:8080/api/v1/ping | Simple ping endpoint |

---

## MinIO (object storage, S3-compatible)

| Service | URL | Credentials |
|--------|-----|-------------|
| **MinIO API** | http://localhost:9000 | S3-compatible API (used by backend for uploads) |
| **MinIO Console** | http://localhost:9001 | Web UI to browse buckets and files |

**Login (Console):**
- User: `minioadmin`
- Password: `minioadmin`

**Default bucket:** `hamsaya-uploads` (created by the app on first use)

**In `.env`:**
- `STORAGE_ENDPOINT=localhost:9000`
- `CDN_URL=http://localhost:9000` (or your CDN in production)

---

## PostgreSQL

| Item | Value |
|------|--------|
| **Host** | localhost |
| **Port** | 5433 (host) → 5432 (container) |
| **Database** | hamsaya |
| **User** | postgres |
| **Password** | postgres |
| **Connection string** | `postgresql://postgres:postgres@localhost:5433/hamsaya?sslmode=disable` |

**CLI (from host):**
```bash
docker exec -it hamsaya-postgres psql -U postgres -d hamsaya
```

---

## Redis

| Item | Value |
|------|--------|
| **Host** | localhost |
| **Port** | 6379 |
| **Password** | (none by default) |
| **URL** | `redis://localhost:6379/0` |

**CLI:**
```bash
docker exec -it hamsaya-redis redis-cli
```

---

## Summary table

| Service | URL or host:port |
|---------|------------------|
| Backend API | http://localhost:8080 |
| Swagger | http://localhost:8080/swagger/index.html |
| MinIO Console | http://localhost:9001 |
| MinIO API | http://localhost:9000 |
| PostgreSQL | localhost:5433 |
| Redis | localhost:6379 |
