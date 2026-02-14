# Hamsaya Backend API

Production-ready Go backend for the Hamsaya mobile application - a social media platform with posts, events, marketplace, business profiles, and real-time messaging.

---

## How to run the backend

Follow these steps to run the API locally.

### 1. Prerequisites

- **Go 1.21+** – [Install Go](https://go.dev/dl/)
- **Docker & Docker Compose** – [Install Docker](https://docs.docker.com/get-docker/)
- **Make** (optional; you can use the equivalent commands below)

### 2. One-time setup

From the project root (`hamsaya_backend_go/`):

```bash
# Copy environment file (use DB_PORT=5433 if Postgres runs via Docker)
cp .env.example .env

# Download Go dependencies
go mod download
# or: make deps
```

### 3. Start infrastructure (PostgreSQL, Redis, MinIO)

```bash
docker-compose up -d postgres redis minio
# or: make docker-up
```

Wait a few seconds for the containers to start.

### 4. Run database migrations

```bash
make migrate-up
# or: go run cmd/migrate/main.go up
```

### 5. Start the API server

```bash
make run
# or: go run cmd/server/main.go
```

The API will be available at **http://localhost:8080**.

### 6. Verify

```bash
curl http://localhost:8080/health
```

Optional: run with **hot reload** during development:

```bash
make install-air   # once
make dev           # run with auto-reload
```

**Full guideline:** See **[docs/RUNNING.md](./docs/RUNNING.md)** for step-by-step instructions, environment notes, troubleshooting, and command reference.

---

## Features

- 🔐 **Authentication**: JWT, OAuth2 (Google/Apple/Facebook), MFA/TOTP
- 👥 **User Management**: Profiles, relationships, privacy settings
- 📱 **Social Features**: Posts (Feed, Event, Sell, Pull), comments, likes, polls
- 🏢 **Business Profiles**: Full business management with categories and hours
- 🛒 **Marketplace**: Sell posts with categories and location-based filtering
- 💬 **Real-time Chat**: WebSocket-based messaging
- 🔔 **Push Notifications**: Firebase Cloud Messaging integration
- 📍 **Location Services**: PostGIS integration for nearby discovery
- 🖼️ **Image Processing**: Upload, resize, compress, WebP conversion
- 📊 **Monitoring**: Structured logging, Prometheus metrics, health checks

## Tech Stack

- **Language**: Go 1.21+
- **Framework**: Gin Web Framework
- **Database**: PostgreSQL 15 with PostGIS extension
- **Cache**: Redis 7
- **Storage**: MinIO (S3-compatible)
- **Authentication**: JWT, OAuth2
- **Logging**: Zap (structured logging)
- **Validation**: go-playground/validator

## Project Structure

```
.
├── cmd/
│   └── server/           # Application entry point
├── config/               # Configuration management
├── internal/
│   ├── handlers/         # HTTP request handlers
│   ├── middleware/       # HTTP middleware
│   ├── models/           # Data models
│   ├── repositories/     # Data access layer
│   ├── services/         # Business logic
│   └── utils/            # Utility functions
├── pkg/
│   ├── database/         # Database connection
│   ├── location/         # Location services
│   ├── notification/     # Push notifications
│   ├── storage/          # Object storage
│   └── websocket/        # WebSocket manager
├── migrations/           # Database migrations
├── docker-compose.yml    # Local development setup
├── Dockerfile           # Production container
└── Makefile            # Common commands
```

## Getting Started

For a short run guide, see **[How to run the backend](#how-to-run-the-backend)** at the top of this file.

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- Make (optional, for convenience)

### Installation

1. **Clone the repository** (or start from this directory)

2. **Copy environment file**

   ```bash
   cp .env.example .env
   ```

3. **Install dependencies**

   ```bash
   go mod download
   # or
   make deps
   ```

4. **Start infrastructure services**

   ```bash
   docker-compose up -d postgres redis minio
   # or
   make docker-up
   ```

5. **Run database migrations**

   ```bash
   make migrate-up
   ```

6. **Run the application**
   ```bash
   go run cmd/server/main.go
   # or
   make run
   ```

The API will be available at `http://localhost:8080`

### Using Docker Compose (Full Stack)

Start all services including the API:

```bash
docker-compose up -d
```

View logs:

```bash
docker-compose logs -f api
```

Stop all services:

```bash
docker-compose down
```

## Development

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# View coverage report
open coverage.html
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint
```

### Database Migrations

```bash
# Create a new migration
make migrate-create name=add_users_table

# Run migrations
make migrate-up

# Rollback last migration
make migrate-down
```

### Hot Reload (Development)

Install Air for hot reload:

```bash
make install-air
```

Run with hot reload:

```bash
make dev
```

## API Documentation

### Health Check Endpoints

- `GET /health` - Basic health check
- `GET /health/live` - Liveness probe (Kubernetes)
- `GET /health/ready` - Readiness probe (checks DB and Redis)
- `GET /health/startup` - Startup probe (Kubernetes)
- `GET /health/db-stats` - Database connection pool statistics
- `GET /health/redis-stats` - Redis server statistics
- `GET /health/version` - Application version and build info
- `GET /health/metrics` - System metrics (memory, CPU, goroutines, uptime)

See [HEALTH_CHECKS.md](./HEALTH_CHECKS.md) for detailed documentation.

### API Endpoints (v1)

All API endpoints are prefixed with `/api/v1`

#### Authentication

- `POST /api/v1/auth/register` - User registration
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/refresh` - Refresh access token
- `POST /api/v1/auth/logout` - User logout
- `POST /api/v1/auth/oauth/google` - Google OAuth
- `POST /api/v1/auth/oauth/apple` - Apple OAuth
- `POST /api/v1/auth/oauth/facebook` - Facebook OAuth
- `POST /api/v1/auth/mfa/enroll` - Enroll in MFA
- `POST /api/v1/auth/mfa/verify` - Verify MFA code

_More endpoints to be added in subsequent phases_

## Configuration

Configuration is managed through environment variables. See `.env.example` for all available options.

Key configurations:

- `SERVER_PORT` - HTTP server port (default: 8080)
- `DB_HOST` - PostgreSQL host
- `REDIS_HOST` - Redis host
- `JWT_SECRET` - JWT signing secret (⚠️ change in production!)
- `STORAGE_ENDPOINT` - MinIO/S3 endpoint (where the server connects to MinIO)
- `CDN_URL` - Base URL for stored files returned to clients. Use `http://localhost:9000` only when the client runs on the same machine. When testing from a mobile device, set to a URL the device can reach (e.g. `http://YOUR_MACHINE_IP:9000`) so image URLs work in the app.

## Deployment

### Building for Production

```bash
# Build optimized production binary
make build-prod

# Build production Docker image
make docker-prod

# Tag and push to registry
make docker-push REGISTRY=your-registry.com

# Deploy to production
make deploy-prod
```

### Security & Quality Checks

```bash
# Run security scanner
make security-scan

# Check for vulnerabilities
make vuln-check

# Run complete CI pipeline
make ci
```

### Database Operations

```bash
# Create database backup
make db-backup DB_URL=postgresql://...

# Restore database
make db-restore DB_URL=postgresql://... BACKUP_FILE=backup.dump

# Check migration status
make migrate-status
```

### Environment Variables

Ensure all required environment variables are set in production:

- Generate a strong `JWT_SECRET` (32+ characters)
- Set `ENV=production`
- Configure external services (OAuth, Firebase, etc.)
- Set up proper CORS origins
- Enable SSL/TLS for database and Redis

See [DEPLOYMENT.md](./DEPLOYMENT.md) for complete production deployment guide.

### Health Checks

The application exposes comprehensive health check endpoints for Kubernetes and monitoring:

- Liveness: `/health/live`
- Readiness: `/health/ready`
- Startup: `/health/startup`
- Metrics: `/health/metrics`

See [HEALTH_CHECKS.md](./HEALTH_CHECKS.md) for complete health check documentation.

## Monitoring

### Logs

The application uses structured logging with Zap:

- All HTTP requests are logged with method, path, status, latency
- Errors are logged with context
- Request IDs are attached to all logs

### Metrics

Prometheus metrics are exposed at `/metrics` (when enabled)

## Contributing

### Code Style

- Follow Go best practices and idioms
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Write tests for new features
- Update documentation

### Commit Messages

Use conventional commits:

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `test:` - Test updates
- `refactor:` - Code refactoring
- `chore:` - Maintenance tasks

## License

Proprietary - All rights reserved

## Support

For issues and questions, please contact the development team.

## Roadmap

- [x] Phase 1: Foundation (database, config, health checks)
- [x] Phase 2: Authentication & User Management (JWT, OAuth, MFA)
- [x] Phase 3: User Profiles & Relationships
- [x] Phase 4: Posts System (Feed, Event, Sell, Pull)
- [x] Phase 5: Social Interactions (comments, likes, polls)
- [x] Phase 6: Business Profiles & Marketplace
- [x] Phase 7: Real-time Chat & WebSockets
- [x] Phase 8: Search & Discovery (full-text search, geospatial)
- [x] Phase 9: Push Notifications (Firebase)
- [x] Phase 10: Production Readiness & Deployment

See [API_DOCUMENTATION.md](./API_DOCUMENTATION.md) for complete API reference.
See [DEPLOYMENT.md](./DEPLOYMENT.md) for production deployment guide.

---

Built with ❤️ using Go and modern best practices

# hamsaya_backend_go
