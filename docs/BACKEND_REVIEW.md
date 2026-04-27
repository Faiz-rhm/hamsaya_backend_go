# Hamsaya Backend — Production Review & Documentation

> Comprehensive technical review and reference documentation for `hamsaya_backend_go/`.
> Generated 2026-04-26 against repo HEAD. Companion to top-level `README.md` and `QUICKSTART.md`.

---

## 1. Project Overview

### Purpose

REST + WebSocket API for the Hamsaya social platform. Serves the Flutter mobile app and Next.js admin panel. Domain features:

- Social feed with four post types (`feed`, `event`, `sell`, `pull` polls)
- User profiles, follow/block relationships, business profiles
- Real-time chat (WebSocket) and notifications (Firebase Cloud Messaging)
- Marketplace (sell posts with auto-expiry, resell flow)
- Events (interested/going state)
- Geospatial search (PostGIS) and full-text search
- Admin moderation (reports, bans, audit logs, broadcast push)
- OAuth2 (Google/Apple/Facebook) + email/password + TOTP MFA

### Tech Stack

| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.25.9 | `go.mod` |
| HTTP framework | Gin v1.12.0 | `github.com/gin-gonic/gin` |
| DB | PostgreSQL 15 + PostGIS 3.3 | Geography(Point, 4326) for location |
| DB driver | pgx v5.5.4 | `jackc/pgx/v5` — raw SQL, no ORM |
| Cache / rate limiter | Redis 7 | `redis/go-redis/v9` |
| Object storage | MinIO (S3-compatible) | `minio/minio-go/v7` |
| Auth | JWT HS256 + bcrypt + TOTP | `golang-jwt/jwt/v5`, `pquerna/otp` |
| WebSocket | gorilla/websocket v1.5.3 | Hub pattern |
| Push | Firebase Admin SDK v4.18 | `firebase.google.com/go/v4` |
| Email | Resend or SMTP | `RESEND_API_KEY` or `SMTP_*` |
| Geocoding | Nominatim / Google | configurable provider |
| Image | imaging + go-webp | WebP encoding on upload |
| Logging | Zap | + OTel zap bridge |
| Metrics | Prometheus | `prometheus/client_golang` |
| Tracing | OpenTelemetry OTLP gRPC | Jaeger compatible |
| Config | Viper + .env | env override priority |
| API docs | swaggo + gin-swagger | `/swagger/index.html` |
| Testing | testify + mockgen | unit + e2e |

### Current Status

MVP-complete, pre-production. Backend serves both clients today. Core surface stable; ~150 routes registered. CI runs unit + e2e on every push (PostGIS service container) plus weekly security scans (gosec, Trivy, govulncheck, Nancy, golangci-lint, dependency review).

---

## 2. Architecture

### Layering

```
HTTP request
    │
    ▼
┌──────────────┐
│ Gin Router   │  cmd/server/main.go (route registration)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Middleware   │  recover → logger → CORS → request_id →
│ chain        │  security headers → body_limit → timeout →
│              │  ban → OTel metrics → OTel tracing →
│              │  (auth + denylist check on protected groups)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Handlers     │  internal/handlers/  (18 files)
│              │  Bind JSON, validate, call service, format response
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Services     │  internal/services/  (28 files)
│              │  Business logic, orchestrates repos + integrations
│              │  Receive *interfaces* for repos → mockable
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Repositories │  internal/repositories/  (25 files)
│              │  Only layer that touches DB; raw parameterized SQL
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────┐
│ pgxpool → PostgreSQL/PostGIS     │
│ Redis (cache, rate limit, tokens)│
│ MinIO (object storage)           │
│ FCM (push)                       │
└──────────────────────────────────┘
```

Strict rule: **services never call DB directly; repositories never call other repositories.** Cross-cutting work (e.g. notify followers on new post) sits in services that compose multiple repos.

### Directory layout

```
cmd/
  server/main.go                    # API entry point
  migrate/main.go                   # golang-migrate runner
  seed/, seed-demo/, seed-admin/    # seeders
  db-reset/                         # data wipe (keeps schema)
  backfill-notifications/           # one-shot backfill job

internal/
  handlers/        # 18 + 18 tests — Gin handlers
  services/        # 28 + 28 tests — business logic
  repositories/    # 25 + 25 tests — SQL access
  middleware/      # 8 + 8 tests — auth, rate limit, CORS, etc.
  models/          # 18 — domain structs + request/response DTOs
  utils/           # validation, response formatting, error codes
  mocks/           # repository_mocks.go (~172 KB, generated)
  testutil/        # db_mock.go, helpers.go

pkg/
  database/        # pgxpool init, transaction helpers
  storage/         # MinIO client, image upload + WebP encode
  notification/    # FCM client + email service
  geocoding/       # Nominatim/Google reverse geocoder
  observability/   # OTel traces/metrics/logs, Prometheus exporter
  websocket/       # gorilla hub, client registry, broadcaster

migrations/        # 33 .up.sql / .down.sql pairs
docs/              # Swagger JSON/YAML, OBSERVABILITY.md, RUNNING.md, LOCAL_URLS.md, this file
tests/e2e/         # 27 end-to-end test files
config/            # Viper-backed config loader
scripts/           # ad-hoc shell helpers
grafana/, prometheus/, nginx/  # ops sidecar configs
```

### Design decisions

- **No ORM.** All queries handwritten with `$1`-style parameters. Trades developer ergonomics for SQL transparency, easier PostGIS use, and predictable performance.
- **Interface-based DI.** Services depend on repository interfaces declared in the service package. Concrete repos satisfy them implicitly. Constructors are plain `NewXService(deps...)` — no Wire/Fx/DI container.
- **Fan-out-on-write for personalized feed.** `FanoutService` denormalizes feed entries per follower at post creation time; trade-off chosen for read latency over write cost. Cache invalidated on follow/unfollow.
- **Graceful degradation for optional integrations.** FCM, OTLP exporter, email, geocoding all start in no-op / log-only mode if credentials are missing. Server starts cleanly without them.
- **Role-based auth via middleware composition.** `RequireAuth → RequireVerifiedEmail → RequireAdmin` are stackable. AAL (Authentication Assurance Level) field in JWT distinguishes password-only (AAL1) from MFA-verified (AAL2) sessions for sensitive ops.
- **WebSocket hub, single-goroutine broadcaster.** Serializes message routing to avoid client-map locking. Acceptable for current scale.

---

## 3. Getting Started

### Prerequisites

- Go 1.25.9
- Docker + Docker Compose
- `make`
- (Optional) `air` for hot reload — `make install-air`
- (Optional) `migrate` CLI if not using `make migrate-up`

### Installation

```bash
git clone <repo>
cd hamsaya_backend_go
cp .env.example .env       # edit secrets
make deps                  # go mod download
make install-tools         # golangci-lint, gosec, air, swag, mockgen
```

### Environment variables

Full list in `.env.example` (91 vars). Required for first boot:

| Var | Purpose | Default |
|---|---|---|
| `SERVER_PORT` | HTTP listen port | `8080` |
| `DB_HOST` / `DB_PORT` / `DB_NAME` / `DB_USER` / `DB_PASSWORD` | Postgres conn | `localhost` / `5433` (Docker) / `hamsaya` / `postgres` / `postgres` |
| `DB_SSL_MODE` | TLS to Postgres | `disable` (dev) |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` | pgxpool sizing | `25` / `5` |
| `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD` / `REDIS_DB` | Redis | `localhost` / `6379` / `` / `0` |
| `JWT_SECRET` | HS256 signing key — **must rotate from default in prod** | `dev-secret-change-in-production-x` |
| `JWT_ACCESS_TOKEN_DURATION` / `JWT_REFRESH_TOKEN_DURATION` | TTLs | `15m` / `720h` |
| `STORAGE_ENDPOINT` / `STORAGE_ACCESS_KEY` / `STORAGE_SECRET_KEY` / `STORAGE_BUCKET_NAME` / `STORAGE_USE_SSL` | MinIO | `localhost:9000` / `minioadmin` / `minioadmin` / `hamsaya-uploads` / `false` |
| `CDN_URL` | URL returned to clients for uploaded media | must be reachable by mobile device — **not `localhost` for physical devices** |
| `CORS_ALLOWED_ORIGINS` | comma-separated origins | `http://localhost:3000,...` |
| `LOG_LEVEL` | `debug` / `info` / `warn` / `error` / `fatal` | `info` |

Optional integrations:

| Var | Required when |
|---|---|
| `RESEND_API_KEY` or `SMTP_*` | Sending real verification/reset emails. If absent, codes print to logs. |
| `FIREBASE_CREDENTIALS_PATH` *or* `FIREBASE_PROJECT_ID` + `FIREBASE_PRIVATE_KEY` + `FIREBASE_CLIENT_EMAIL` | Push notifications. Absent → push disabled. |
| `GEOCODING_API_KEY` + `GEOCODING_PROVIDER` | Reverse geocoding. Absent → Nominatim public endpoint (rate-limited). |
| `GOOGLE_CLIENT_ID` / `_SECRET`, `APPLE_*`, `FACEBOOK_APP_*` | OAuth providers per-platform. |
| `OTLP_ENDPOINT` + `OBSERVABILITY_ENABLED=true` | OTel export to Jaeger/Tempo. |
| `SENTRY_DSN` | Error reporting. |

### Run locally

```bash
make docker-up         # postgres + redis + minio
make migrate-up        # apply 33 migrations
make seed-demo         # comprehensive demo data (or: make seed)
make run               # → http://localhost:8080
# OR
make dev               # hot reload via air
```

### Run with Docker

```bash
# Dev
docker-compose up -d

# Prod (requires env overrides)
docker-compose -f docker-compose.prod.yml up -d

# + Observability stack (Jaeger)
docker-compose -f docker-compose.observability.yml up -d
```

The `Dockerfile` is multi-stage (golang:1.25.9-alpine → alpine:latest), runs as non-root user, includes healthcheck against `/health`, and bundles `migrations/` so the container can self-migrate at boot if wired.

### Local URLs

| Service | URL |
|---|---|
| API base | http://localhost:8080 |
| Swagger UI | http://localhost:8080/swagger/index.html |
| Health (ready) | http://localhost:8080/health/ready |
| Prometheus metrics | http://localhost:8080/metrics |
| MinIO Console | http://localhost:9001 (minioadmin / minioadmin) |
| PostgreSQL | localhost:5433 (postgres / postgres) |
| Redis | localhost:6379 |
| Jaeger UI (if observability up) | http://localhost:16686 |

---

## 4. API Documentation

Base URL: `http://localhost:8080/api/v1`
Auth: `Authorization: Bearer <access_token>` on protected routes.
OpenAPI: `docs/swagger.yaml` and `docs/swagger.json`. Live UI at `/swagger/index.html`. Regenerate with `make docs`.

### Standard response envelope

```json
{
  "success": true,
  "message": "User registered successfully",
  "data":   { ... },
  "error":  null
}
```

Error responses keep the same shape with `success: false`, an error code (e.g. `INVALID_JSON`, `VALIDATION`, `UNAUTHORIZED`, `RATE_LIMITED`), and a human message.

### Authentication

Three middleware tiers:

| Middleware | Effect |
|---|---|
| (none) | Public route |
| `RequireAuth` | Validates JWT, loads user, blocks suspended/banned users |
| `RequireVerifiedEmail` | `RequireAuth` + email verified flag |
| `RequireAdmin` | `RequireAuth` + admin role |
| `RequireAAL2` | Requires MFA-verified session for sensitive ops |

JWT claims:

```json
{
  "user_id":    "<uuid>",
  "email":      "<email>",
  "aal":        1,
  "session_id": "<uuid>",
  "iat":        1714000000,
  "exp":        1714000900,
  "iss":        "hamsaya"
}
```

### Endpoint catalog (selected)

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/auth/register` | rate-limited | Email/password signup |
| POST | `/auth/login` | rate-limited | Email/password login |
| POST | `/auth/unified` | rate-limited | Combined login/register flow used by mobile |
| POST | `/auth/refresh` | refresh token | Issue new access + refresh pair |
| POST | `/auth/forgot-password` | strict (3/5min) | Send reset code via email |
| POST | `/auth/verify-reset-code` | strict (3/10min) | Verify OTP from email |
| POST | `/auth/reset-password` | strict | Set new password using verified OTP |
| POST | `/auth/mfa/verify` | RequireAuth | Submit TOTP, upgrades session AAL → 2 |
| POST | `/auth/oauth/{google,facebook,apple}` | rate-limited | OAuth2 / Sign in with Apple |
| POST | `/auth/logout` | RequireAuth | Invalidate current session |
| POST | `/auth/logout-all` | RequireAuth | Invalidate all sessions |
| GET | `/users/me` | RequireAuth | Current user profile |
| PUT | `/users/me` | RequireAuth | Update profile |
| DELETE | `/users/me` | RequireAuth | Soft-delete account |
| POST | `/users/me/avatar` | RequireAuth | Multipart upload |
| GET | `/users/:user_id` | RequireAuth | Public profile |
| POST | `/users/:user_id/follow` | RequireAuth | Follow |
| POST | `/users/:user_id/block` | RequireAuth | Block |
| POST | `/posts` | RequireVerifiedEmail | Create post (`type` ∈ feed/event/sell/pull) |
| GET | `/posts/feed` | RequireAuth | Personalized feed (fan-out cache) |
| GET | `/posts/:post_id` | RequireAuth | Get post + engagement |
| PUT | `/posts/:post_id` | RequireVerifiedEmail | Edit post / update poll options |
| DELETE | `/posts/:post_id` | RequireVerifiedEmail | Delete post |
| POST | `/posts/:post_id/like` | RequireAuth | Like |
| POST | `/posts/:post_id/bookmark` | RequireAuth | Bookmark |
| POST | `/posts/:post_id/share` | RequireAuth | Share |
| POST | `/posts/:post_id/resell` | RequireVerifiedEmail | Clone SELL post under current user |
| POST | `/posts/:post_id/comments` | RequireVerifiedEmail | Comment |
| GET | `/posts/:post_id/polls` | RequireAuth | Poll for PULL post |
| POST | `/polls/:poll_id/vote` | RequireVerifiedEmail | Vote |
| GET | `/events/:post_id/interest` | RequireAuth | My interest state |
| POST | `/events/:post_id/interest` | RequireVerifiedEmail | Set interested/going |
| GET | `/events/:post_id/{interested,going}` | RequireAuth | List users |
| POST | `/businesses` | RequireAuth | Create business profile |
| GET | `/businesses` | RequireAuth | Search/list |
| POST | `/businesses/:id/follow` | RequireAuth | Follow |
| GET | `/categories` | RequireAuth | Sell categories (multi-language) |
| GET | `/chat/ws` | RequireVerifiedEmail | WebSocket upgrade |
| POST | `/chat/messages` | RequireVerifiedEmail | Send message |
| GET | `/chat/conversations` | RequireVerifiedEmail | List threads |
| GET | `/chat/conversations/:id/messages` | RequireVerifiedEmail | History |
| GET | `/notifications` | RequireAuth | Inbox |
| POST | `/notifications/fcm-token` | RequireAuth | Register FCM token |
| GET | `/search` | RequireAuth | Unified search |
| GET | `/search/{posts,users,businesses,discover}` | RequireAuth | Faceted search |
| POST | `/feedback` | RequireVerifiedEmail | Submit feedback |
| POST | `/help-chat/messages` | RequireAuth | Support chat |
| GET | `/admin/stats` | RequireAdmin | Dashboard counters |
| GET | `/admin/analytics/{users,posts,engagement,businesses}` | RequireAdmin | Charts |
| POST | `/admin/users/:id/suspend` | RequireAdmin | Suspend user |
| POST | `/admin/notifications/broadcast` | RequireAdmin | FCM broadcast |
| GET | `/admin/audit-logs` | RequireAdmin | Audit trail |
| POST | `/admin/ip-bans`, `/admin/device-bans` | RequireAdmin | Ban management |
| GET | `/health/{,live,ready,startup,db-stats,redis-stats,version,metrics}` | none | Probes |
| GET | `/metrics` | none | Prometheus scrape |
| GET | `/swagger/*any` | none | OpenAPI UI |

Full canonical list: regenerate Swagger (`make docs`) and consult `/swagger/index.html`.

### Example: register

```http
POST /api/v1/auth/register
Content-Type: application/json
X-Device-Info: ios-iphone15-osVersion=17.4

{
  "email":    "alice@example.com",
  "password": "Str0ng!Pass",
  "full_name":"Alice Example",
  "username": "alice"
}
```

```json
{
  "success": true,
  "message": "User registered successfully",
  "data": {
    "user": { "id": "...", "email": "...", "username": "...", "email_verified": false },
    "tokens": {
      "access_token":  "eyJhbGciOi...",
      "refresh_token": "eyJhbGciOi...",
      "expires_at":    "2026-04-26T16:30:00Z"
    },
    "session_id": "..."
  }
}
```

### Pagination / filtering

List endpoints accept `?limit=&offset=` (or cursor for feed). Response includes `data.items` plus `data.pagination = { total, limit, offset, has_more }`. Filters expressed as query params (e.g. `?type=sell&category_id=...&country=AF`).

---

## 5. Database Schema

Driver: pgx v5. Pool init in `pkg/database/database.go`:

```go
poolConfig.MaxConns        = cfg.MaxConns          // 25
poolConfig.MinConns        = cfg.MinConns          // 5
poolConfig.MaxConnLifetime = cfg.MaxConnLifetime   // 1h
poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime   // 30m
```

Geo columns use `GEOGRAPHY(POINT, 4326)`. Inserts via `ST_SetSRID(ST_MakePoint($lng, $lat), 4326)::geography`. Reads project to lng/lat with `ST_X(col::geometry)::double precision`, `ST_Y(col::geometry)::double precision`.

### High-level ER (text)

```
users ─┬─< user_sessions
       ├─< mfa_secrets
       ├─< user_roles
       ├─< follows >─ users          (self-referential)
       ├─< blocks  >─ users
       ├─< posts ─┬─< post_attachments
       │           ├─< post_likes / post_bookmarks / post_shares
       │           ├─< comments ─< comment_likes (recursive parent_id)
       │           ├─< polls ─< poll_options ─< poll_votes
       │           └─< event_interests
       ├─< business_profiles ─┬─< business_categories
       │                       ├─< business_hours
       │                       ├─< business_attachments
       │                       └─< business_followers
       ├─< conversations >─ users (2-party) ─< messages
       ├─< notifications (+ notification_settings)
       ├─< reports (post|comment|user|business)
       ├─< feedback
       └─< fcm_tokens
admin_invites, audit_logs, ip_bans, device_bans   (admin domain)
help_chat_threads ─< help_chat_messages           (support)
fanout_feed (denormalized per-user feed cache)
```

### Migrations

- Tool: `golang-migrate` (custom Go entry at `cmd/migrate/main.go`)
- 33 migration pairs, sequentially numbered `20240101000NNN_*.{up,down}.sql`
- Order matters; gated extensions (`postgis`, `uuid-ossp`) come first
- Run: `make migrate-up` / `make migrate-down` / `make migrate-status`
- New migration: `make migrate-create name=add_foo_table` → creates timestamped `.up.sql` + `.down.sql`

Production tip: use `CREATE INDEX CONCURRENTLY` for indexes on large tables; `golang-migrate` does not wrap each migration in a single transaction by default for files containing `CONCURRENTLY`.

### Seeders

- `make seed` — basic
- `make seed-demo` — comprehensive (recommended for local UI work)
- `make seed-sell-categories` — categories only, no wipe
- `make db-reset` — truncates all tables, retains schema

---

## 6. Testing

### Layout

```
internal/handlers/<x>_handler_test.go        # handler-level unit tests
internal/services/<x>_service_test.go        # service unit tests against mocks
internal/repositories/<x>_repository_test.go # repo tests
internal/middleware/<x>_test.go              # middleware tests
internal/mocks/repository_mocks.go           # generated repository mocks
internal/testutil/{db_mock.go,helpers.go}    # shared fixtures + harness
tests/e2e/                                   # 27 black-box flow tests
```

### Run tests

```bash
make test              # go test -v -race ./...
make test-coverage     # + HTML coverage at coverage.html
go test -v -race -run TestRegister_Success ./internal/services/   # single test
go test -v -race -timeout=5m ./tests/e2e/...                       # e2e only
```

E2E suite needs the Docker stack up (`make docker-up`) and migrations applied. CI runs against an ephemeral `postgis/postgis:15-3.3` service container with DB name `hamsaya_test`.

### Mocking strategy

- Repository interfaces defined in service packages → service tests inject mocks from `internal/mocks/`.
- DB-touching code lives only in repos → repo tests run against a real Postgres (CI service container) or `internal/testutil/db_mock.go` for lightweight cases.
- External services (FCM, MinIO, email, geocoder) interact via small interfaces; tests inject fakes.

### Coverage goals

| Layer | Target | Notes |
|---|---|---|
| services | ≥ 80% | core business logic |
| handlers | ≥ 70% | mostly bind/validate/forward — kept thinner |
| repositories | ≥ 70% | SQL paths, including PostGIS branches |
| middleware | ≥ 90% | small files, security-critical |
| utils | ≥ 90% | response, error codes, validation |
| pkg/* | best effort | integration-heavy (websocket, FCM) |

### What's missing / recommended

- **Load tests.** No `k6`/`vegeta` scripts checked in. Recommend adding a `tests/load/` profile for the feed and chat endpoints before scale events.
- **Contract tests against admin panel + mobile.** Swagger spec is the contract today; consider Pact or schema-diff CI.
- **Mutation testing.** Optional but valuable for service layer (`go-mutesting`).
- **Property-based tests.** `gopter` for poll vote tallying, fanout consistency.
- **Chaos / failover tests.** Redis flap, Postgres failover behavior currently unverified.

---

## 7. Security

### Authentication & authorization
- JWT HS256 signed with `JWT_SECRET`. **Default secret in `.env.example` and `docker-compose.yml` must be replaced in any deployed env.** Production deployments use `docker-compose.prod.yml`, which requires `JWT_SECRET` to be supplied externally.
- Refresh tokens stored server-side in Redis with TTL; `/auth/logout` invalidates the active token, `/auth/logout-all` invalidates all sessions for the user.
- TOTP MFA via `pquerna/otp`. Session AAL field gates sensitive operations (`RequireAAL2`).
- Bcrypt cost 12 (production); cost 4 in tests for speed.
- Password complexity enforced in `password_service.go`: ≥8 chars, upper, lower, digit, special.
- OAuth (Google, Facebook, Apple) verified server-side against provider's public keys / token endpoints.

### Authorization
- Three role tiers: `user`, `moderator`, `admin`. Admin panel calls require `admin` or `moderator`. Mobile-only flows require `user`.
- Resource ownership checked in services (e.g., a user can only edit their own post).
- Admin actions written to `audit_logs` table.

### Input validation
- `gin.Context.ShouldBindJSON()` + custom validator wrapper around `go-playground/validator/v10`. Tags: `required`, `email`, `min`, `max`, etc.
- File uploads checked for content type and size in handlers / `pkg/storage`.
- All SQL parameterized via pgx (`$1`, `$2`, ...). No string concatenation observed in sampled repos.

### Secrets management
**Verified (2026-04-26 audit):**

1. ✅ `serviceAccountKey.json` is **not** tracked. Already in `.gitignore` (`serviceAccountKey.json` and `*serviceAccount*.json`). History scan (`git log --all -S "BEGIN PRIVATE KEY"`) found no commit introducing key material — only references to env-var-driven FCM construction in `pkg/notification/fcm.go`.
2. ✅ `.env` is **not** tracked. Already in `.gitignore`. Only `.env.example` (placeholder values) is committed.
3. ⚠️ **No secret manager integration.** For production, externalize `JWT_SECRET`, `DB_PASSWORD`, `FIREBASE_*`, `RESEND_API_KEY` to AWS Secrets Manager / GCP Secret Manager / Vault rather than relying on `docker-compose.prod.yml` env passthrough.
4. ⚠️ **`server_bin` was tracked as a Mach-O binary** until 2026-04-26; removed via `git rm --cached server_bin` and added to `.gitignore`. Old commits still contain the artifact — safe (no secrets baked in beyond the `service_account` literal string), but consider purging via `git filter-repo` to shrink repo size.

### Network hardening
- Security headers middleware sets CSP, HSTS (when TLS detected), X-Frame-Options DENY, X-Content-Type-Options nosniff, Referrer-Policy strict-origin-when-cross-origin, Permissions-Policy (geolocation/camera/microphone disabled).
- CORS driven by `CORS_ALLOWED_ORIGINS` (comma-separated). Verified non-`*` in `.env.example`.
- No HTTPS termination in-process — expect a reverse proxy (nginx config bundled, or Cloudflare/ALB).

### Rate limiting
Redis token-bucket (`INCR` + `EXPIRE`). Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`.

| Bucket | Limit | Applied to |
|---|---|---|
| `default` | 100/min | global fallback |
| `auth` | 5/min | `/auth/register`, `/auth/login`, `/auth/unified` |
| `strict` | 3/5min | `/auth/forgot-password` |
| `password-reset` | 3/10min | `/auth/verify-reset-code`, `/auth/reset-password` |
| `reports` | 10/24h | `/.../report` endpoints |

### Dependency scanning
CI runs on every push and weekly:
- **gosec** — Go static security analysis (SARIF → GitHub Security tab)
- **Trivy** — filesystem CVE scan
- **govulncheck** — official Go vulnerability DB
- **Nancy** — Sonatype SCA against `go.sum`
- **GitHub Dependency Review** — fails PRs introducing high-severity advisories
- **golangci-lint** — broad lint suite

Direct dependencies are current as of 2026-04. No flagged abandoned libraries.

### Known gaps / recommended fixes
- Rotate Firebase service account; remove `serviceAccountKey.json` from VCS history (`git filter-repo`).
- Confirm `.env` is gitignored; rotate the dev `JWT_SECRET` and DB password if either ever pointed at prod.
- Add max-length validation on long text fields (post description, comment body) at handler layer; rely on DB length where feasible.
- Standardize generic auth error messages (`"invalid credentials"`) to avoid user-enumeration via error text.
- Consider per-user (not just per-IP) limits on `/posts` POST to mitigate authenticated abuse.
- Add CAPTCHA or proof-of-work to `/auth/register` for production.

---

## 8. Deployment

### Build

```bash
make build              # → bin/server (CGO required for libwebp)
make build-prod         # production-tuned build
make docker-prod        # multi-arch image via Dockerfile.prod
```

`Dockerfile` is multi-stage (`golang:1.25.9-alpine` builder → `alpine:latest` runtime), runs as `app:1000` non-root, ships migrations alongside the binary, and includes a HEALTHCHECK against `/health`.

### Environments

| File | Purpose |
|---|---|
| `docker-compose.yml` | Dev — Postgres+PostGIS, Redis, MinIO, API |
| `docker-compose.prod.yml` | Prod — required env overrides, resource limits, localhost-only DB/Redis bindings, Postgres tuning (max_connections=200, shared_buffers=256MB), Redis AOF+RDB w/ 512MB cap |
| `docker-compose.observability.yml` | Optional — Jaeger all-in-one (16686 UI, 4317 OTLP gRPC, 4318 OTLP HTTP) |

### CI/CD

- `.github/workflows/test.yml` — push + PR. PostGIS service container, Redis, full `go test -v -race -timeout=5m ./...`.
- `.github/workflows/security.yml` — push + PR + weekly cron. gosec, Trivy, govulncheck, Nancy, golangci-lint, dependency-review.
- **No CD pipeline.** Recommend adding a deployment workflow (GitHub Actions → registry push → server pull/restart, or Kubernetes manifests + ArgoCD). At minimum, a tagged-release Docker image build.

### Operational checklist (production)
- [ ] `JWT_SECRET` ≥ 32 chars, externally managed
- [ ] `DB_SSL_MODE=require` and certificate pinned
- [ ] `STORAGE_USE_SSL=true` with real S3 / MinIO behind TLS
- [ ] `CORS_ALLOWED_ORIGINS` lists prod domains only
- [ ] `CDN_URL` points at public CDN, not the MinIO host
- [ ] `OBSERVABILITY_ENABLED=true` + reachable OTLP collector
- [ ] FCM credentials via env or mounted secret (not file in repo)
- [ ] Reverse proxy (nginx/ALB/Cloudflare) terminates TLS, forwards `X-Forwarded-For`
- [ ] DB backups + point-in-time recovery configured
- [ ] Redis persistence acceptable for data role (rate-limit only? token storage too?)
- [ ] Healthcheck wired to load balancer (`/health/ready`)

---

## 9. Monitoring & Logging

### Structured logging
- `go.uber.org/zap` sugared logger throughout
- Level controlled by `LOG_LEVEL`
- Per-request fields: `request_id`, `method`, `path`, `status`, `latency`, `client_ip`, `user_id` (when auth'd)
- Correlated with traces via OTel zap bridge (`otelzap`) when observability enabled

### Metrics (Prometheus, `/metrics`)
- `http_requests_total{method,route,status}` — counter
- `http_request_duration_seconds{method,route}` — histogram (5ms..10s buckets)
- `http_server_active_requests` — gauge
- `http_request_size_bytes`, `http_response_size_bytes` — histograms
- `db_query_duration_seconds`, `db_query_total` — DB observability
- Domain counters: `users_created`, `posts_created`, `messages_created`, `active_websockets`

Grafana dashboards under `grafana/`, Prometheus config under `prometheus/`.

### Tracing
- OpenTelemetry OTLP gRPC exporter, configurable sampling (`OTLP_SAMPLING_RATE`, default 1.0)
- Gin instrumented via `otelgin.Middleware`
- Trace IDs threaded into log records

### Health checks
| Path | Use |
|---|---|
| `/health` | basic 200 |
| `/health/live` | liveness probe |
| `/health/ready` | readiness — checks DB + Redis connectivity |
| `/health/startup` | startup probe (initialization complete) |
| `/health/db-stats` | pgxpool stats (connections in use, idle, max) |
| `/health/redis-stats` | Redis pool stats |
| `/health/version` | git commit/build info |
| `/health/metrics` | mirror of `/metrics` |

### Background jobs
Two long-running goroutines started in `cmd/server/main.go`:
1. **Sell-post expiry** — every 1h, immediately on boot. Marks SELL posts past their expiry as expired, optionally notifies seller.
2. **Session cleanup** — every 24h. Purges expired `user_sessions` rows.

No external scheduler (no cron container). Acceptable for single-instance deployments; for multi-instance, add a leader election (Redis lock) before scaling.

---

## 10. Known Issues & TODOs

### Code-level TODOs (grep)
- `comment_service.go` — TODO: extract richer photo metadata from storage on upload.
- `mfa_service.go` — formatting comment for 8-character codes (not actionable).

Repo is unusually clean of TODO/FIXME/HACK markers — substantive work tracked outside source.

### Issues surfaced by this review

| Severity | Item | Status |
|---|---|---|
| ✅ resolved | `serviceAccountKey.json` / `.env` exposure | Verified gitignored + never committed |
| ✅ resolved | `server_bin` Mach-O binary tracked | Removed from index; gitignore updated |
| ✅ resolved | Auth error messages enable user enumeration | Standardized to generic `invalid credentials` |
| ✅ resolved | Background jobs not leader-elected | Redis SETNX leader lock added |
| ✅ resolved | Silent OTLP fallback | Startup WARN when exporter init fails |
| ✅ resolved | Long text fields lack max-length validation | `validate:"max=N"` tags added on hot request types |
| ✅ resolved | No per-user rate limit on POST /posts | Per-user-id bucket added |
| ✅ resolved | Access token replayable after `/auth/logout` | Redis JTI denylist + middleware check |
| ✅ resolved | No global request body size cap | `BodyLimit(5 MB)` middleware |
| ✅ resolved | Hung queries holding DB connections | `Timeout(25s)` middleware on all requests |
| ✅ resolved | No CD pipeline | `release.yml` workflow: tagged image push to ghcr (multi-arch + SBOM) |
| ✅ resolved | No nightly fuzz CI | `fuzz.yml` workflow runs FuzzAuthHandler_Register weekly |
| ✅ resolved | No load tests | `tests/load/{feed,post_create}.js` k6 skeleton |
| ✅ resolved | gosec findings | 0 issues; 6 documented `#nosec` annotations |
| 🟡 med | No external secret manager | Deferred — env vars + rotation plan accepted |
| 🟢 low | MFA secrets not encrypted at rest | Deferred |
| 🟢 low | WebSocket hub single-broadcaster goroutine | Split per-shard if concurrency exceeds ~10k connections |
| 🟢 low | Image upload pipeline is synchronous (encode WebP in-request) | Move to async worker if upload latency becomes user-visible |
| 🟢 low | Connection pool default `MaxConns=25` may be low under load | Monitor `/health/db-stats`, raise if saturated |
| 🟢 low | `server_bin` in old commit history | Declined — skip `git filter-repo` |

### Recent feature additions (April 2026)

| Area | Change | Files |
|---|---|---|
| Anti-abuse | **Daily post limits** per type, admin-editable. Redis-keyed counter with UTC-midnight TTL + 30s in-process limit cache. Mobile renders `DailyLimitBadge` with countdown to reset; admin Next.js page CRUDs caps. | `migrations/...daily_post_limits...up.sql`, `internal/services/daily_limit_service.go`, `internal/handlers/daily_limit_handler.go`, mobile `lib/src/featured/post/provider/daily_limit_provider.dart`, admin `app/(dashboard)/daily-limits/page.tsx` |
| Upload safety | **Per-file 25 MB image cap** enforced via `utils.EnforceUploadSize` at six upload sites (post, avatar, profile cover, three business image fields). Mobile mirrors the cap and rejects oversized crops before compression. | `internal/utils/upload.go`, `internal/handlers/{post,profile,business}_handler.go`, mobile `lib/src/featured/profile/provider/profile_uploader_provider.dart` |
| UX guardrail | **Comment depth-3 cap.** `CreateComment` walks the parent chain and returns 400 when nesting would exceed 3 levels. | `internal/services/comment_service.go` |
| Mobile UX | **Avatar / cover cropping** via `image_cropper` (1:1 / 16:9). Cancel-aware; crops piped through existing WebP compression and upload pipeline. | mobile `lib/src/featured/profile/provider/profile_uploader_provider.dart`, `pubspec.yaml` |
| Profile UX | **Profile completion %** + missing-fields list shipped on `GET /profile/me`. Mobile renders `ProfileCompletionBar` (auto-hides at 100%) on the user's own profile; tap deep-links to edit screen. | `internal/services/profile_service.go`, `internal/models/profile.go`, mobile `lib/src/featured/profile/widgets/profile_completion_bar.dart` |
| Feed quality | **Suppress unpromoted SELL posts on home feed.** `FeedFilter.HideUnpromotedSell` flag short-circuits in repo + fan-out service so non-promoted marketplace posts stay in the marketplace tab. | `internal/repositories/post_repository.go`, `internal/services/post_fanout_service.go` |
| Ops | **Daily Postgres dump → S3** sidecar with 7-day retention. Runs from `db-backup` service in `docker-compose.prod.yml`. | `scripts/backup_postgres.sh`, `docker-compose.prod.yml` |

---

## 11. Future Recommendations

### Scaling
- **Horizontal scale:** stateless API behind a load balancer. Pre-reqs: leader election for background jobs, shared Redis for sessions/rate limit (already), sticky sessions only if WebSocket clients can't reconnect cleanly.
- **WebSocket scale-out:** today single-process hub. For multi-instance, introduce a Redis pub/sub fanout layer or migrate to a managed channel service (NATS, Centrifugo).
- **Read replicas:** route feed and search reads to a Postgres replica via `DB_REPLICA_*` env. Repository layer would need a small router.
- **PostGIS performance:** confirm GIST indexes on every `geography` column referenced in WHERE/ORDER BY (`migrations/20240101000007_create_indexes.up.sql` should be audited as data grows).
- **Fanout backpressure:** current fan-out-on-write writes `O(followers)` rows per post. For high-follower accounts, switch to fan-out-on-read or hybrid (pull for celebrity accounts).
- **CDN for media:** terminate uploads at MinIO/S3 but serve via CloudFront/Cloudflare/Bunny. `CDN_URL` already abstracts this.

### Feature additions
- **Soft-delete + purge job** for posts/comments to support moderation rollback windows.
- **Account export (GDPR Article 20)** endpoint for user data download.
- **Webhook system** for business profile events (booking integrations).
- **API key auth** for trusted server-to-server (e.g. partner integrations).
- **Server-Sent Events** as a fallback for environments where WebSocket is blocked.

### Performance optimizations
- Add `EXPLAIN (ANALYZE, BUFFERS)` baselines for the top-5 hottest queries (feed, search, conversation-list, notifications, business-list) into a `docs/PERF_BASELINE.md`.
- Cache `GET /categories` and `GET /businesses?...` (read-heavy, mutate-rarely) in Redis with tag-based invalidation.
- Move WebP encoding off the request path into a worker; return original URL immediately and progressively replace with WebP.
- Tune pgxpool `MaxConns` proportionally to (instance count × Postgres `max_connections / instances`).
- Use `pgxpool.Conn.PgConn().Prepare` for the hottest queries (the feed query is the obvious candidate).

### Operational
- Add `/admin/debug/pprof` (auth-gated) for runtime profiling.
- Add structured error codes in OpenAPI so the mobile client can map to localized strings.
- Migrate Swagger to OpenAPI 3.1, generate typed clients for the admin panel and mobile app on every release.
- Track DORA metrics (deploy frequency, MTTR) once the CD pipeline lands.

---

## Appendix A — Reference

- Top-level `README.md`, `QUICKSTART.md`, `POLL_OPTIONS_UPDATE.md`
- `docs/RUNNING.md` — full setup walkthrough
- `docs/OBSERVABILITY.md` — OTel/Prometheus/Jaeger
- `docs/LOCAL_URLS.md` — local service URLs cheat-sheet
- `docs/IMPLEMENTATION_GUIDES.md` — feature implementation notes
- `docs/swagger.yaml` / `swagger.json` — OpenAPI spec
- `Makefile` — exhaustive command list (`make help`)

## Appendix B — Quick command reference

```bash
# Daily dev
make docker-up && make migrate-up && make seed-demo && make dev

# Tests
make test
make test-coverage   # coverage.html
go test -v -race -run <Name> ./internal/services/

# Code quality
make fmt             # gofmt -s
make lint            # golangci-lint
make security-scan   # gosec

# DB lifecycle
make migrate-create name=add_foo_table
make migrate-up
make migrate-down
make migrate-status
make db-reset

# Docs
make docs            # regenerate Swagger from annotations
```
