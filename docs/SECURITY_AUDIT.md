# Security & Test Audit Report

> Date: 2026-04-26
> Scope: `hamsaya_backend_go/` (Go 1.25.9, Gin, pgx, Redis, MinIO, JWT, FCM)
> Inputs: full repo scan, `go test -race -cover ./...`, `govulncheck`, `gosec`
> Companion to: `BACKEND_REVIEW.md`

---

## Executive summary

- **0** critical or high-severity vulnerabilities currently exploitable.
- **3** transitive-dependency CVEs identified by `govulncheck` — **all upgraded** (`pgx v5.5.4 → v5.9.0`, `grpc v1.79.2 → v1.79.3`).
- **6** `gosec` findings — 3 false positives (fnv hash never errors), 2 path-traversal false positives in trusted migration loader, 1 documented design choice (fire-and-forget goroutine using `context.Background`).
- **Test suite: 100% passing** across 10 packages (was: 2 failing packages on entry — `internal/handlers` and `tests/e2e`). Both root causes diagnosed and fixed.
- **5 new tests added** in 3 files: redislock primitive, e2e SQLi sweep, e2e logout/token contract, e2e oversized-password rejection, fuzz target on `/auth/register`.

---

## Part 1 — Security vulnerability assessment

### Authentication & session

| Check | Present? | Severity | Notes / Fix |
|---|---|---|---|
| Weak password policy | No | — | `password_service.go` enforces min 8 chars + upper + lower + digit + special |
| Account lockout after failed attempts | Yes | — | `MaxLoginAttempts=5` → 30-min lock in `auth_service.go` |
| Session fixation | No | — | New `session_id` issued on every successful login; refresh rotates server-side |
| Insecure session storage | No | — | Tokens in `Authorization` header; refresh in Redis with TTL |
| Logout invalidates tokens | **Yes** | — | Refresh token revoked server-side; **access token also revoked** via Redis JTI denylist (`/auth/logout` writes JTI for remaining TTL; `extractAndValidateToken` checks on every request). Pinned by `TestE2E_Security_AccessTokenAfterLogout` (asserts 401 after logout) and unit tests `TestRequireAuth_DenylistedTokenRejected` / `TestRequireAuth_LegacyTokenWithoutJTIStillValid` (backwards compat). |
| Token leakage in logs / responses | No | — | Zap logger does not log password or token fields; debug logs disabled in prod |
| JWT `alg=none` vulnerability | No | — | `jwt.SigningMethodHS256` enforced; library v5 rejects `none` by default |
| JWT missing expiration | No | — | `exp` claim set on every token; access 15m, refresh 720h |
| Refresh-token rotation | Yes | — | Old refresh token deleted from Redis on each `/auth/refresh` |
| Account-enumeration via login error | **Fixed** | High → resolved | Locked-account branch previously returned distinct error; now returns generic `Invalid email or password`. Tests updated. |
| Bcrypt-DoS via huge password | **Fixed** | Medium → resolved | `validate:"max=128"` added to all password fields in `models/auth.go` |

### Authorization

| Check | Present? | Severity | Notes |
|---|---|---|---|
| IDOR — horizontal privilege escalation | No | — | Service layer ownership checks (`comment.UserID != userID` → `ForbiddenError`). Covered by `TestE2E_Post_DeleteByNonOwnerReturns403`, `TestCommentHandler_DeleteComment/not_owner` |
| RBAC | Yes | — | Roles: `user` / `moderator` / `admin`. `RequireAdmin` middleware on `/admin/*` |
| Vertical privilege escalation | No | — | All admin endpoints gated; no role assignable from user-controlled input |
| MFA-gated sensitive ops | Yes | — | `RequireAAL2` available; AAL claim in JWT distinguishes password-only from MFA-verified sessions |

### Input validation

| Check | Present? | Severity | Notes |
|---|---|---|---|
| SQL injection | No | — | All queries parameterized via pgx (`$1`, `$2`); zero string-concat sites in core repos. New e2e test `TestE2E_Security_SQLInjection_Search` exercises 6 classic payloads against the search endpoint |
| NoSQL injection | N/A | — | Postgres only |
| Command injection | No | — | No `exec.Command` invocations on user input |
| XSS in API responses | N/A | — | JSON-only responses; clients (mobile + admin) escape on render. No `Content-Type: text/html` paths |
| Path traversal | Low | Low | `pkg/database/migrate.go` reads files via `filepath.Walk` from a fixed migrations dir; `gosec` G304 flag is a false positive — input is build-time controlled |
| XXE / LDAP / buffer overflow | N/A | — | Go runtime + JSON parser; no XML, no LDAP |
| Integer overflow | No | — | Pagination limits clamped; no untrusted arithmetic on numeric inputs |
| Mass assignment | No | — | DTOs are explicit structs; admin-only fields (Role, EmailVerified) never bound from user requests |
| Max-length enforcement | **Improved** | Medium → resolved | `validate:"max=N"` tags now cover password (128), token (4096), email (320), device_info (512), description (5000), text (1000), feedback (2000), help-chat content (2000) |

### API security

| Check | Present? | Severity | Notes |
|---|---|---|---|
| Rate limiting on auth | Yes | — | 5/min on `/auth/{register,login,unified}`, 3/5min on `/auth/forgot-password`, 3/10min on OTP verify |
| Rate limiting on writes | **Improved** | Medium → resolved | Per-user `LimitPostsCreate()` (30/hour) added to `POST /posts` and `POST /posts/upload-image` |
| Rate limit on reports | Yes | — | 10/24h per user via `LimitReports()` |
| CORS restrictions | Yes | — | `CORS_ALLOWED_ORIGINS` (comma-separated, not `*`) |
| API versioning | Yes | — | All routes under `/api/v1` |
| Sensitive data in URL params | No | — | Tokens in `Authorization` header; passwords in body |
| HTTP method override | No | — | Gin default; no `_method` override middleware |

### Data security

| Check | Present? | Severity | Notes |
|---|---|---|---|
| Plaintext passwords | No | — | bcrypt cost 12 (cost 4 in tests for speed) |
| Sensitive data in logs | No | — | Spot-checked; no `password`/`token` keys in zap structured logs |
| Encryption at rest | Deferred | Medium | Application-level encryption for MFA secrets recommended; depends on Postgres TDE / cloud KMS |
| Encryption in transit | Conditional | Medium | TLS terminated upstream (nginx/ALB/Cloudflare); `STORAGE_USE_SSL` configurable |
| DB credentials in code | No | — | Env-only via Viper |
| API keys hardcoded | No | — | All in env / `.env` (gitignored) |
| Backup exposure | N/A | — | No backup paths exposed by API |

### Security headers

| Header | Set? | Source |
|---|---|---|
| `X-Content-Type-Options: nosniff` | Yes | `internal/middleware/security.go` |
| `X-Frame-Options: DENY` | Yes | same |
| `Content-Security-Policy: default-src 'self'; frame-ancestors 'none'` | Yes | same |
| `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload` | Yes (when TLS) | same |
| `X-XSS-Protection: 1; mode=block` | Yes | same |
| `Cache-Control: no-store, no-cache, must-revalidate` | Yes | same — applied globally |
| `Referrer-Policy: strict-origin-when-cross-origin` | Yes | same |
| `Permissions-Policy: geolocation=(), microphone=(), camera=()` | Yes | same |

### Dependency vulnerabilities

`govulncheck` initial run:

| Advisory | Module | Severity | Found | Fixed |
|---|---|---|---|---|
| GO-2026-4772 (CVE-2026-33816) | `github.com/jackc/pgx/v5` | Medium | v5.5.4 | **Upgraded → v5.9.0** |
| GO-2026-4771 (CVE-2026-33815) | `github.com/jackc/pgx/v5` | Medium | v5.5.4 | **Upgraded → v5.9.0** |
| GO-2026-4762 | `google.golang.org/grpc` | High (auth bypass) | v1.79.2 | **Upgraded → v1.79.3** |

Post-upgrade: **`govulncheck`: "No vulnerabilities found."**

Note: `govulncheck` reported all three were not in the call graph (transitive only), so no exploit was reachable. Upgraded anyway as defense in depth.

### Static analysis (gosec)

**Issues: 0** (Files: 118, Lines: 47626, Nosec: 6 documented annotations).

Resolved during this audit:

| Rule | Severity | Location | Resolution |
|---|---|---|---|
| G104 ×3 | Low | `post_service.go:74`, `comment_service.go:23`, `business_service.go:690` | Explicit `_, _ = h.Write(...)` (`fnv.Hash.Write` never errors per Go docs). |
| G115 ×2 | High | `config/config.go:188-189` | Added `getInt32(key)` helper that clamps to `[math.MinInt32, math.MaxInt32]` before conversion. |
| G122 + G304 | High / Medium | `pkg/database/migrate.go:104` | Migration loader rewritten to use `os.DirFS` + `fs.WalkDir` + `fs.ReadFile`, scoping all access under `migrationsPath`. |
| G118 | High | `business_service.go:164` (was 161) | Documented annotation: `go func() { // #nosec G118 -- detached on purpose }`. The goroutine intentionally uses `context.Background()` so `IncrementViews` outlives the request. |
| G404 | High | `models/user.go:52` | Documented annotation: random avatar color is a UI-only attribute, weak randomness is intentional. |
| G101 ×4 | High | `pkg/notification/fcm.go:43-49`, `internal/testutil/helpers.go:13`, `internal/services/email_service.go:306, 410` | Documented annotations. Each is a false positive: FCM credential **field names** in a JSON map (not values), a unit-test bcrypt hash for the literal string `"password"`, and HTML email templates whose copy contains the word "password". |

### Infrastructure

| Check | Status | Notes |
|---|---|---|
| Debug endpoints in prod | Safe | No `/debug/pprof` mounted; `/metrics` is intentional and behind ingress |
| Build info exposure | Minimal | `/health/version` returns commit SHA — useful for ops, not exploitable |
| Health endpoints leak | No | `/health/db-stats` returns counts only, not config |
| Request timeouts | Yes | `srv.ReadTimeout=15s`, `WriteTimeout=15s`, `IdleTimeout=60s`, `MaxHeaderBytes=1MB` |
| Body size limits | Partial | Multipart upload limited at MinIO layer; recommend explicit Gin `MaxMultipartMemory` cap |
| TLS configuration | Out of scope | Terminated upstream |

---

## Part 2 — Test inventory & gap analysis

### Inventory (post-audit)

| Layer | Files | Tests | Coverage |
|---|---|---|---|
| `config` | 1 | — | 51.7% |
| `internal/handlers` | 19 | unit + 1 fuzz | 40.8% |
| `internal/middleware` | 7 | unit (rate limit, auth, CORS, headers, ban) | 71.1% |
| `internal/services` | 24 | unit (mock-repo) | 51.8% |
| `internal/repositories` | 19 | unit (limited; most need real DB → e2e) | 27.0% |
| `internal/utils` | 4 | unit | 57.1% |
| `pkg/geocoding` | 1 | unit | 18.9% |
| `pkg/redislock` | 1 (new) | unit (incl. concurrent) | **73.7%** |
| `pkg/websocket` | 1 | unit | 55.4% |
| `tests/e2e` | 28 (was 27; +1 security_test.go) | full-stack | n/a |

**Total test files:** 108 (was 103 on entry). All pass under `go test -race`.

This pass added: `body_limit_test.go`, `timeout_test.go`, JTI tests in `jwt_service_test.go`, denylist tests in `auth_test.go`. Earlier audit passes added `pkg/redislock/lock_test.go`, `tests/e2e/security_test.go`, `internal/handlers/auth_handler_fuzz_test.go`.

### Existing critical-test coverage

| Test type | Status | Where |
|---|---|---|
| Edge cases (nil, empty) | ✅ Present | every `*_test.go` table-driven cases |
| Race detector | ✅ Run on every CI build | `make test` uses `-race` |
| Rate-limit enforcement | ✅ Present | `internal/middleware/rate_limit_test.go` (5 tests) |
| Token expiration / refresh | ✅ Present | `auth_service_test.go`, `TestE2E_AuthFlow_RegisterLoginRefreshLogout` |
| Logout invalidates refresh | ✅ Present | step 5 of `TestE2E_AuthFlow_RegisterLoginRefreshLogout` |
| CORS policy | ✅ Present | `internal/middleware/cors_test.go` |
| Graceful shutdown | ✅ Present | implicit via `cmd/server/main.go` signal handling; e2e suites use `httptest` cleanup |
| Database transaction | ✅ Present | repository tests cover `CreateUserWithProfile` (atomic) |
| WebSocket | ✅ Present | `pkg/websocket/*_test.go` |

### Gaps closed by this audit

| Test | File | Verifies |
|---|---|---|
| `TestAcquire_Succeeds_WhenKeyFree` | `pkg/redislock/lock_test.go` | Basic acquire/release |
| `TestAcquire_Contention_SecondCallerFails` | same | Mutual exclusion |
| `TestAcquire_AfterTTLExpiry_AnotherCallerWins` | same | Lock expiry semantics |
| `TestRelease_DoesNotDeleteOtherHoldersToken` | same | Safe-release script (no token-stealing) |
| `TestAcquire_ConcurrentCallers` | same | Exactly-one-winner under 50-goroutine contention |
| `TestE2E_Security_SQLInjection_Search` | `tests/e2e/security_test.go` | 6 SQLi payloads → no 5xx |
| `TestE2E_Security_AccessTokenAfterLogout` | same | Pins current logout-token contract; refresh token rejected |
| `TestE2E_Security_OversizedPasswordRejected` | same | bcrypt-DoS guard via `max=128` validator tag |
| `FuzzAuthHandler_Register` | `internal/handlers/auth_handler_fuzz_test.go` | No-panic contract on arbitrary JSON input to `/auth/register` |

### Remaining gaps (low priority)

| Type | Status | Why deferred |
|---|---|---|
| Load tests (k6) | ✅ Skeleton present | `tests/load/feed.js`, `post_create.js`, README. Run against staging — not production |
| Mutation testing (`go-mutesting`) | ❌ | Optional quality gate |
| Property-based tests (gopter) | ❌ | Useful for poll vote tally; deferred |
| Chaos / failover (Redis flap) | ❌ | Needs container orchestration; out of scope here |
| Contract tests against admin/mobile | ❌ | OpenAPI is the contract today; consider Pact in CI |

---

## Part 3 — Test run results

```
go test -race -cover -timeout=300s ./...

ok  	github.com/hamsaya/backend/config              coverage: 51.7%
ok  	github.com/hamsaya/backend/internal/handlers   coverage: 40.8%
ok  	github.com/hamsaya/backend/internal/middleware coverage: 71.1%
ok  	github.com/hamsaya/backend/internal/repositories coverage: 27.0%
ok  	github.com/hamsaya/backend/internal/services   coverage: 51.8%
ok  	github.com/hamsaya/backend/internal/utils      coverage: 57.1%
ok  	github.com/hamsaya/backend/pkg/geocoding       coverage: 18.9%
ok  	github.com/hamsaya/backend/pkg/redislock       coverage: 73.7%
ok  	github.com/hamsaya/backend/pkg/websocket       coverage: 55.4%
ok  	github.com/hamsaya/backend/tests/e2e
```

```
govulncheck ./...
=== Symbol Results ===
No vulnerabilities found.

Your code is affected by 0 vulnerabilities.
```

```
gosec ./...
Files: 116, Lines: 47479, Issues: 6 (3 false positives, 3 documented)
```

### Failures fixed during audit

| Test | Symptom | Root cause | Fix |
|---|---|---|---|
| `TestCommentHandler_DeleteComment/not_owner` | expected 401, got 403 | Test asserted wrong status — handler correctly returns 403 Forbidden for ownership violation | Updated assertion to `http.StatusForbidden` |
| `TestPostHandler_DeletePost/not_post_owner` | expected 401, got 403 | Same — test wrong, handler correct | Updated assertion |
| `TestE2E_Search_Posts` | `failed to scan post: number of field descriptions must equal number of destinations, got 42 and 41` | `SearchPosts` SQL used `SELECT DISTINCT p.*`; the `search_vector` migration added a 40th column without updating the scan target | Replaced `p.*` with explicit column list mirroring `scanArgs` order |

---

## Part 4 — New tests added

| File | Tests | Purpose |
|---|---|---|
| `pkg/redislock/lock_test.go` | 5 | Cover the new leader-election primitive: acquire, contention, TTL expiry, safe release, 50-goroutine race |
| `tests/e2e/security_test.go` | 3 (multi-subtest SQLi) | SQLi sweep on search; logout-vs-access-token contract; bcrypt-DoS guard |
| `internal/handlers/auth_handler_fuzz_test.go` | 1 fuzz target | No-panic contract for `/auth/register` on arbitrary JSON. Run via `go test -fuzz=FuzzAuthHandler_Register ./internal/handlers/` |

---

## Part 5 — Final report

### Security findings (consolidated)

| Severity | Issue | Status | Resolution |
|---|---|---|---|
| 🔴 High | Locked-account login leaks existence (account enumeration) | ✅ Fixed (prior pass) | Both `Login` and `UnifiedAuth` paths now return generic `Invalid email or password` |
| 🔴 High | gRPC auth-bypass CVE in transitive dep | ✅ Fixed | `google.golang.org/grpc v1.79.2 → v1.79.3` |
| 🟡 Medium | pgx CVEs (×2) in transitive call paths | ✅ Fixed | `pgx v5.5.4 → v5.9.0` |
| 🟡 Medium | bcrypt-DoS via unbounded password input | ✅ Fixed (prior pass) | `validate:"max=128"` |
| 🟡 Medium | No per-user rate limit on `POST /posts` | ✅ Fixed (prior pass) | `LimitPostsCreate()` 30/hour |
| 🟡 Medium | Background jobs not leader-elected | ✅ Fixed (prior pass) | Redis SETNX leader lock + tests |
| 🟡 Medium | Silent OTLP fallback | ✅ Fixed (prior pass) | Startup WARN/INFO on init outcome |
| 🟡 Medium | `server_bin` Mach-O committed | ✅ Fixed (prior pass) | `git rm --cached`, gitignored |
| 🟡 Medium | Long-text fields lack max-length | ✅ Fixed (prior pass) | Validator tags added across auth + content models |
| 🔴 High | Access token replayable after `/auth/logout` until 15-min expiry | ✅ Fixed | Redis JTI denylist + middleware check; e2e + unit tests pin behavior |
| 🟡 Medium | No global request-body size cap (payload-bomb DoS) | ✅ Fixed | `BodyLimit` middleware (5 MB default) on all requests |
| 🟡 Medium | Hung repository query holding DB connection forever | ✅ Fixed | `Timeout` middleware (25s) on every request; WebSocket upgrade exempted |
| 🟡 Medium | gosec G115 int overflow in `config.go` | ✅ Fixed | `getInt32` clamp helper |
| 🟡 Medium | gosec G122/G304 path-traversal on migration loader | ✅ Fixed | `os.DirFS` scope |
| 🟢 Low | gosec false positives (G101, G104, G118, G404) | ✅ Documented | Inline `#nosec` annotations with justification |
| 🟡 Medium | No external secret manager | Open (deferred) | User decision: env vars + rotation plan for now |
| 🟢 Low | `server_bin` in git history | Open (declined) | User decision: skip `git filter-repo` |
| 🟢 Low | MFA secrets not encrypted at rest | Open (deferred) | User decision |
| 🟢 Low | Background jobs use `context.Background()` (bypass request timeout) | Acceptable design | User decision: jobs must outlive request lifecycle |

### Test coverage summary

- **Total test files:** 108
- **Packages passing:** 10 / 10
- **Failing:** 0
- **Skipped:** 0 (e2e auto-skip when DB unreachable; all ran in this audit)
- **gosec issues:** 0 (6 documented `#nosec` annotations)
- **govulncheck:** No vulnerabilities found
- **Aggregate package coverage:** ~50% (range: 18.9% geocoding → 73.7% redislock; e2e provides additional black-box coverage uncounted here)

### Recommendations (prioritized)

Items resolved this session:
- ✅ **CD pipeline** — `.github/workflows/release.yml` builds + pushes multi-arch image to ghcr on `v*` tag with provenance + SBOM.
- ✅ **Gin body-size cap** — `BodyLimit(5 MB)` middleware on all requests.
- ✅ **Nightly fuzz CI** — `.github/workflows/fuzz.yml` runs `FuzzAuthHandler_Register` 5 min weekly.
- ✅ **Request timeouts** — `Timeout(25s)` middleware applies a deadline globally; repos inherit via `context.WithTimeout`.
- ✅ **Access-token denylist** — JTI-keyed Redis denylist on `/auth/logout`.
- ✅ **`migrate.go` hardening** — `os.DirFS` scope.
- ✅ **Load test skeleton** — `tests/load/{feed,post_create}.js`.

Items still open (deferred per user decision):
1. **External secret manager** — env vars + rotation plan accepted for now. Revisit when scaling beyond a single deploy or before regulated workloads.
2. **MFA secret encryption at rest** — deferred. If/when adopted, wrap `mfa_secrets.secret` with KMS-backed envelope encryption.
3. **`server_bin` purge from git history** — declined.

Items where the current design is the chosen answer (not gaps):
- Background jobs (`ProcessExpiredSellPosts`, session cleanup) intentionally use `context.Background()` so they survive request lifecycles. Leader-elected via Redis SETNX.
- 15-min access-token TTL with denylist is the chosen logout semantics; no token rotation on every request.

---

## Appendix — Commands used

```bash
# Full test sweep
go test -race -cover -timeout=300s ./...

# Vulnerability scan (after upgrade)
$HOME/go/bin/govulncheck ./...

# Static security analysis
$HOME/go/bin/gosec -quiet -fmt=text ./...

# Fuzz (seed-corpus-only)
go test -count=1 -run "^FuzzAuthHandler_Register$" ./internal/handlers/

# Fuzz (active fuzzing — run locally / nightly CI)
go test -run=^$ -fuzz=FuzzAuthHandler_Register -fuzztime=5m ./internal/handlers/
```
