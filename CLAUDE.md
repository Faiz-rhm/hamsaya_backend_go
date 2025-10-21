# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Hamsaya Backend** is a production-ready Go backend for a social media mobile application. It provides:

- Social features: posts (4 types: FEED, EVENT, SELL, PULL), comments, likes, polls
- User management: authentication (JWT, OAuth, MFA), profiles, relationships
- Business profiles: full business management with categories, hours, followers
- Marketplace: sell posts with categories and location-based filtering
- Real-time features: WebSocket chat, push notifications
- Location services: PostGIS integration for nearby discovery

## Technology Stack

- **Language**: Go 1.21+
- **Web Framework**: Gin
- **Database**: PostgreSQL 15 with PostGIS extension
- **Cache**: Redis 7
- **Storage**: MinIO (S3-compatible) for images
- **Logging**: Zap (structured logging)
- **Testing**: testify for assertions

## Project Structure

```
.
├── cmd/
│   ├── server/           # Main application entry point (cmd/server/main.go)
│   └── migrate/          # Database migration CLI tool
├── config/               # Configuration management (Viper-based)
├── internal/             # Private application code
│   ├── handlers/         # HTTP request handlers (Gin handlers)
│   ├── middleware/       # HTTP middleware (logging, CORS, auth, etc.)
│   ├── models/           # Data models and structs
│   ├── repositories/     # Data access layer (database operations)
│   ├── services/         # Business logic layer
│   └── utils/            # Utility functions (logger, errors, response, validator)
├── pkg/                  # Public, reusable packages
│   ├── database/         # Database connection and migration system
│   ├── location/         # Location services (geocoding, distance calc)
│   ├── notification/     # Push notifications (Firebase)
│   ├── storage/          # Object storage (MinIO/S3)
│   └── websocket/        # WebSocket manager
├── migrations/           # SQL migration files (format: YYYYMMDDHHMMSS_name.up/down.sql)
├── docker-compose.yml    # Local development environment
├── Dockerfile           # Production container
└── Makefile            # Common development commands
```

## Architecture Patterns

### Layered Architecture

The codebase follows a clean architecture pattern with clear separation of concerns:

1. **Handlers Layer** (`internal/handlers/`): HTTP request/response handling
   - Parse requests
   - Call services
   - Return formatted responses

2. **Services Layer** (`internal/services/`): Business logic
   - Orchestrate operations
   - Handle transactions
   - Implement business rules

3. **Repositories Layer** (`internal/repositories/`): Data access
   - Database queries
   - CRUD operations
   - No business logic

4. **Models** (`internal/models/`): Data structures
   - Request/response DTOs
   - Domain entities
   - Database models

### Key Design Decisions

- **No ORM**: Uses `pgx` directly for better performance and control
- **Transaction handling**: Services manage transactions, repositories work within them
- **Dependency injection**: Pass dependencies through constructors
- **Error handling**: Use custom `AppError` type with HTTP status codes
- **Logging**: Structured logging with request IDs for tracing
- **Validation**: Use `validator` tags on structs

## Common Development Commands

```bash
# Build
make build                  # Build the server binary
go build -o bin/server cmd/server/main.go

# Run
make run                    # Run the server locally
make dev                    # Run with hot-reload (requires air)

# Docker
make docker-up              # Start all services (PostgreSQL, Redis, MinIO, API)
make docker-down            # Stop all services
make docker-logs            # View logs
docker-compose up -d postgres redis minio  # Start only infrastructure

# Database migrations
make migrate-up             # Apply all pending migrations
make migrate-down           # Rollback last migration
make migrate-create name=add_users_table  # Create new migration
make migrate-status         # Check migration status

# Testing
make test                   # Run all tests
make test-coverage          # Run tests with coverage report
go test ./internal/... -v   # Run specific package tests
go test -run TestFunctionName ./path/to/package
make benchmark              # Run performance benchmarks

# Code quality
make lint                   # Run linter
make fmt                    # Format code

# Production
make build-prod             # Build optimized production binary
make docker-prod            # Build production Docker image
make docker-push REGISTRY=your-registry.com  # Tag and push to registry
make deploy-prod            # Deploy to production
make security-scan          # Run security scanner (gosec)
make vuln-check             # Check for vulnerabilities
make db-backup DB_URL=...   # Create database backup
make db-restore DB_URL=... BACKUP_FILE=...  # Restore database
make health-check           # Check application health

# CI/CD
make ci                     # Run complete CI pipeline (lint, test, build)
make ci-test                # Run tests with coverage
make ci-lint                # Run linter
make ci-build               # Build binary
```

## Database Migrations

### Creating Migrations

Always create both up and down migrations:

```bash
make migrate-create name=add_users_table
# Creates:
# - migrations/20231215123456_add_users_table.up.sql
# - migrations/20231215123456_add_users_table.down.sql
```

### Migration Guidelines

- Use transactions where appropriate (most migrations)
- Add indexes in separate migrations if they take time
- Always test down migrations
- Include appropriate constraints (NOT NULL, FOREIGN KEY, etc.)
- Use `IF EXISTS` / `IF NOT EXISTS` for idempotency where possible

### Running Migrations

```bash
# Apply all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Check status
bin/migrate status
```

## Configuration

Configuration is managed via environment variables (see `.env.example`). The `config/config.go` file loads and validates all settings.

### Key Configuration Areas

- **Server**: Port, host, environment, log level
- **Database**: Connection, pool size, timeouts
- **Redis**: Connection details
- **JWT**: Secret, token durations
- **OAuth**: Google, Apple, Facebook credentials
- **Storage**: MinIO/S3 configuration
- **Rate Limiting**: Request limits
- **CORS**: Allowed origins, methods, headers

### Environment Setup

```bash
# Copy example environment file
cp .env.example .env

# Edit .env with your values
# Ensure JWT_SECRET is strong in production
```

## Testing Patterns

### Unit Tests

- Place tests next to the code: `file.go` → `file_test.go`
- Use `testify/assert` for assertions
- Mock external dependencies (use interfaces)
- Test edge cases and error paths

Example:
```go
func TestUserService_CreateUser(t *testing.T) {
    // Arrange
    mockRepo := new(mocks.UserRepository)
    service := NewUserService(mockRepo)

    // Act
    user, err := service.CreateUser(ctx, req)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, user)
    mockRepo.AssertExpectations(t)
}
```

### Integration Tests

- Use `_integration_test.go` suffix
- Set up test database
- Clean up after tests

## HTTP Response Format

All API responses follow a consistent format using `utils.Response`:

```go
// Success
utils.SendSuccess(c, http.StatusOK, "User created", user)
// Returns: {"success": true, "message": "User created", "data": {...}}

// Error
utils.SendError(c, http.StatusBadRequest, "Invalid input", err)
// Returns: {"success": false, "message": "Invalid input", "error": "..."}

// Paginated
utils.SendPaginated(c, items, page, limit, totalCount)
// Returns: {"success": true, "data": [...], "meta": {"page": 1, ...}}
```

## Error Handling

Use custom error types from `utils/errors.go`:

```go
// Create application errors with proper HTTP status
return utils.NewBadRequestError("Invalid email", err)
return utils.NewNotFoundError("User not found", err)
return utils.NewUnauthorizedError("Invalid token", err)

// Or use predefined errors
if user == nil {
    return utils.ErrUserNotFound
}
```

## Logging

Use structured logging with context:

```go
logger := utils.GetLogger()

// Info logging
logger.Infow("User created",
    "user_id", user.ID,
    "email", user.Email,
)

// Error logging
logger.Errorw("Failed to create user",
    "error", err,
    "email", req.Email,
)

// Include request ID from context
requestID := c.GetString("request_id")
logger.Infow("Processing request", "request_id", requestID)
```

## Database Access Patterns

### Using Repositories

```go
// Create repository
userRepo := repositories.NewUserRepository(db)

// Simple queries
user, err := userRepo.GetByID(ctx, userID)
users, err := userRepo.List(ctx, page, limit)

// Create/Update
err := userRepo.Create(ctx, user)
err := userRepo.Update(ctx, user)
```

### Transactions

Services handle transactions:

```go
func (s *PostService) CreatePost(ctx context.Context, req *CreatePostRequest) error {
    tx, err := s.db.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    // Use transaction in repository calls
    if err := s.postRepo.CreateTx(ctx, tx, post); err != nil {
        return err
    }

    if err := s.attachmentRepo.CreateTx(ctx, tx, attachment); err != nil {
        return err
    }

    return tx.Commit(ctx)
}
```

## Adding New Endpoints

Follow this pattern:

1. **Create Model** (internal/models/)
   ```go
   type CreateUserRequest struct {
       Email    string `json:"email" validate:"required,email"`
       Password string `json:"password" validate:"required,min=8"`
   }
   ```

2. **Create Repository** (internal/repositories/)
   ```go
   type UserRepository interface {
       Create(ctx context.Context, user *models.User) error
   }
   ```

3. **Create Service** (internal/services/)
   ```go
   type UserService struct {
       userRepo repositories.UserRepository
   }
   ```

4. **Create Handler** (internal/handlers/)
   ```go
   func (h *UserHandler) Create(c *gin.Context) {
       var req CreateUserRequest
       if err := c.ShouldBindJSON(&req); err != nil {
           utils.SendBadRequest(c, "Invalid request", err)
           return
       }
       // ... call service ...
       utils.SendCreated(c, "User created", user)
   }
   ```

5. **Register Route** (cmd/server/main.go)
   ```go
   v1.POST("/users", userHandler.Create)
   ```

## Database Schema Reference

The complete database schema is documented in `GO_BACKEND_IMPLEMENTATION_PLAN.md`. Key tables:

- `users`, `profiles` - User management
- `posts`, `attachments` - Post system (4 types: FEED, EVENT, SELL, PULL)
- `post_comments`, `comment_attachments` - Comments with nesting
- `polls`, `poll_options`, `user_polls` - Polling system
- `business_profiles`, `business_hours` - Business management
- `conversations`, `messages` - Chat system
- `notifications`, `notification_settings` - Notifications
- `user_sessions`, `mfa_factors` - Authentication

## Important Notes

### PostGIS Integration

- Use `GEOGRAPHY(POINT, 4326)` for location columns
- Use `ST_Distance` for distance calculations (returns meters)
- Use `ST_DWithin` for radius queries
- Indexes: `CREATE INDEX ... USING GIST(location)`

Example:
```sql
SELECT * FROM posts
WHERE ST_DWithin(
    address_location,
    ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography,
    5000  -- 5km radius in meters
);
```

### Authentication Flow

1. **JWT Tokens**:
   - Access token: 15 minutes
   - Refresh token: 7 days
   - Include in header: `Authorization: Bearer <token>`

2. **AAL Levels**:
   - AAL1: Basic auth (email/password, OAuth)
   - AAL2: MFA verified (required for sensitive operations)

3. **Session Management**:
   - Track in `user_sessions` table
   - Support multiple devices
   - Implement token rotation

### Image Upload Pattern

```go
// 1. Validate image
// 2. Process (resize, crop, compress)
// 3. Convert to WebP
// 4. Upload to MinIO
// 5. Return Photo struct with URL, dimensions, size

photo, err := storageService.UploadImage(ctx, file, ImageTypeAvatar)
```

### WebSocket Chat Pattern

The application uses WebSocket for real-time chat messaging:

1. **WebSocket Hub**: Central manager in `pkg/websocket/hub.go`
   - Manages connected clients
   - Routes messages between users
   - Handles client registration/unregistration

2. **Connection Flow**:
   ```go
   // Client connects to /api/v1/ws
   // Authenticate via JWT in query param or header
   // Upgrade HTTP to WebSocket
   // Register client with hub
   ```

3. **Message Format**:
   ```json
   {
     "type": "message|typing|read_receipt",
     "conversation_id": "uuid",
     "content": "message content",
     "timestamp": "2024-01-01T00:00:00Z"
   }
   ```

4. **Usage in Handlers**:
   ```go
   // Upgrade connection
   conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)

   // Create client
   client := websocket.NewClient(hub, conn, userID)

   // Register and start
   hub.Register(client)
   go client.WritePump()
   go client.ReadPump()
   ```

## Current Implementation Status

✅ **All Phases Complete - Production Ready**

**Phase 1 - Foundation:**
- Project structure and dependencies
- Docker Compose for local development
- Database connection with connection pooling (pgx)
- Migration system (9 migrations applied)
- Health check endpoints (/health, /health/ready, /health/live, /health/startup, /health/db-stats, /health/redis-stats, /health/version, /health/metrics)
- Structured logging (Zap)
- Error handling (custom AppError types)
- Request validation (validator v10)
- CORS and middleware stack
- Request ID tracking

**Phase 2 - Authentication & User Management:**
- Complete database schema (users, profiles, sessions, MFA, etc.)
- JWT authentication (access + refresh tokens)
- Email/password registration and login
- Email verification system
- Password reset flow
- Change password functionality
- OAuth integration (Google, Apple, Facebook)
- MFA/TOTP enrollment and verification
- MFA backup codes
- Session management (multiple devices)
- Token blacklisting
- Account lockout (failed login attempts)
- Rate limiting (Redis-based)

**Phase 3 - User Profiles:**
- Profile CRUD operations
- Avatar and cover photo upload
- Image processing (resize, compress, WebP conversion)
- MinIO/S3 storage integration
- Get user profile by ID
- Update profile (upsert)
- Privacy settings support

**Phase 4 - User Relationships:**
- Follow/unfollow users
- Block/unblock users
- Get followers/following lists
- Relationship status checking
- User report functionality

**Phase 5 - Posts System:**
- Post CRUD operations (create, get, update, delete)
- Support for all 4 post types (FEED, EVENT, SELL, PULL)
- Location-based posts (PostGIS integration)
- Post engagement (like, unlike, bookmark, unbookmark, share)
- Feed with filtering (by type, user, category, location)
- Post attachments (photos/videos)
- Original post enrichment for shared posts
- Comment system with nested replies
- Comment attachments and likes
- Poll system for PULL posts (create, vote, change vote, delete vote)
- Poll result calculations with percentages
- Event interests (interested, going, not_interested states)
- Event participant lists with pagination

**Phase 6 - Business Profiles:**
- Business profile CRUD operations
- Business categories (many-to-many relationship)
- Business hours management (operating hours by day)
- Business gallery (multiple photos)
- Avatar and cover photo upload
- Follow/unfollow businesses
- Business search with filters (location, category, province, keyword)
- Business info enrichment in posts
- Location-based business discovery (PostGIS radius search)

**Phase 7 - Marketplace Categories:**
- Category CRUD operations
- Post categories (sell posts, marketplace filtering)
- Category management endpoints
- Admin category management

**Phase 8 - Real-time Chat:**
- WebSocket-based chat system
- Conversation CRUD operations
- Real-time message delivery
- Message history and pagination
- Online/offline status tracking
- Typing indicators support
- Read receipts

**Phase 9 - Push Notifications:**
- Firebase Cloud Messaging integration
- Notification CRUD operations
- Notification settings management
- Push notification delivery
- In-app notification system
- Notification preferences by type

**Phase 10 - Search & Production Readiness:**
- Full-text search (users, posts, businesses)
- Location-based search with radius
- Advanced filtering and sorting
- Production Docker configuration
- Enhanced health checks and metrics
- Comprehensive documentation (API, Deployment, Health Checks)
- Security scanning and vulnerability checks
- Database backup/restore procedures

**Implemented Services:**
- AuthService (complete)
- JWTService (complete with tests)
- PasswordService (complete with tests)
- EmailService (complete)
- MFAService (complete)
- OAuthService (complete)
- ProfileService (complete)
- RelationshipsService (complete)
- StorageService (complete with MinIO/S3)
- TokenStorageService (Redis-based, complete)
- PostService (complete)
- CommentService (complete)
- PollService (complete)
- EventService (complete)
- BusinessService (complete)
- CategoryService (complete)
- ChatService (complete)
- NotificationService (complete)
- SearchService (complete)

**Implemented Handlers:**
- HealthHandler (complete with 8 endpoints)
- AuthHandler (complete)
- MFAHandler (complete)
- OAuthHandler (complete)
- ProfileHandler (complete)
- RelationshipsHandler (complete)
- PostHandler (complete)
- CommentHandler (complete)
- PollHandler (complete)
- EventHandler (complete)
- BusinessHandler (complete)
- CategoryHandler (complete)
- ChatHandler (complete with WebSocket)
- NotificationHandler (complete)
- SearchHandler (complete)

**Database:**
- All tables created (users, profiles, posts, comments, business profiles, categories, conversations, messages, notifications)
- PostGIS extension enabled
- Comprehensive indexes including full-text search
- Database triggers for counter management
- Helper functions for distance calculations
- 9 migrations applied

See `GO_BACKEND_IMPLEMENTATION_PLAN.md` for the complete implementation roadmap and `PHASE_10_COMPLETION.md` for production readiness details.

## Debugging Tips

1. **Check logs**: All operations are logged with structured data
2. **Use request IDs**: Every request has a unique ID in logs and responses
3. **Database queries**: Enable query logging by setting log level to debug
4. **Health checks**: Comprehensive endpoints available:
   - `/health` - Basic health check
   - `/health/ready` - Readiness probe (checks DB + Redis)
   - `/health/live` - Liveness probe
   - `/health/startup` - Startup probe for Kubernetes
   - `/health/db-stats` - Database connection pool statistics
   - `/health/redis-stats` - Redis server statistics
   - `/health/version` - Application version and build info
   - `/health/metrics` - System metrics (memory, CPU, goroutines, uptime)
5. **Monitoring**: See `HEALTH_CHECKS.md` for complete monitoring setup

## Common Pitfalls to Avoid

- ❌ Don't use ORMs - use pgx directly
- ❌ Don't skip validation - always validate input
- ❌ Don't forget transactions for multi-step operations
- ❌ Don't log sensitive data (passwords, tokens)
- ❌ Don't hardcode secrets - use environment variables
- ❌ Don't skip error handling - always check and handle errors
- ❌ Don't create repositories without interfaces - use interfaces for testability
- ❌ Don't put business logic in handlers - keep it in services

## Performance Considerations

- Use connection pooling (already configured)
- Add database indexes for frequently queried columns
- Use Redis for caching hot data
- Implement pagination for list endpoints
- Use prepared statements (pgx does this automatically)
- Monitor slow queries with logging
- Use `EXPLAIN ANALYZE` for query optimization

## Security Best Practices

- Always validate and sanitize user input
- Use parameterized queries (prevent SQL injection)
- Hash passwords with bcrypt (cost factor 12)
- Implement rate limiting on auth endpoints
- Use HTTPS in production
- Validate JWT tokens on every protected endpoint
- Implement CORS properly
- Never expose internal error details to clients
- Use secure session management
- Implement account lockout after failed login attempts

## Useful SQL Queries for Development

```sql
-- Check applied migrations
SELECT * FROM schema_migrations ORDER BY version;

-- Check connection count
SELECT count(*) FROM pg_stat_activity;

-- Check table sizes
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename))
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Find slow queries (requires pg_stat_statements extension)
SELECT query, calls, mean_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
```

## Additional Documentation

For comprehensive details on specific topics, refer to these documentation files:

- **`API_DOCUMENTATION.md`** - Complete API reference with endpoints, request/response examples, authentication flows
- **`DEPLOYMENT.md`** - Production deployment guide with Docker, Kubernetes, monitoring, security
- **`HEALTH_CHECKS.md`** - Detailed health check documentation and monitoring setup
- **`GO_BACKEND_IMPLEMENTATION_PLAN.md`** - Complete implementation roadmap and technical specifications
- **`PHASE_10_COMPLETION.md`** - Production readiness completion summary
- **`STORAGE_IMPLEMENTATION.md`** - MinIO/S3 storage implementation details
- **`MISSING_FEATURES.md`** - Known TODO items and incomplete features
- **`SECURITY_FIXES_IN_PROGRESS.md`** - Security improvements and fixes

## Known Limitations & TODOs

Before going to production, review `MISSING_FEATURES.md` for:

1. **Admin Role Middleware** - Admin routes need role-based access control
2. **Session Verification** - Auth middleware needs session validation
3. **WebSocket Origin Checking** - WebSocket connections need proper origin validation
4. **Swagger Documentation** - Generate API documentation with `make swagger`

These items are documented but not yet critical for basic operation.
