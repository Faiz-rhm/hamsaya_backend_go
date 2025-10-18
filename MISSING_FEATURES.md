# Missing & Incomplete Features

This document outlines features that are missing, incomplete, or have TODO items that need attention before going to production.

**Status**: Updated October 16, 2025

---

## ✅ Recently Implemented

### 1. ~~Actual S3/MinIO Storage Integration~~ ✅ **IMPLEMENTED**

**Status**: ✅ **FULLY IMPLEMENTED** (October 16, 2025)

**What Was Done**:
- ✅ Created full MinIO/S3 client in `pkg/storage/storage.go`
- ✅ Implemented actual upload to storage bucket
- ✅ Implemented image processing (resize, crop, compress)
- ✅ Implemented delete from storage
- ✅ CDN URL support
- ✅ Error handling and graceful fallback
- ✅ Smart fallback to mock storage when not configured

**Files Created**:
- `pkg/storage/storage.go` (321 lines) - MinIO client
- `pkg/storage/image.go` (187 lines) - Image processing
- `STORAGE_IMPLEMENTATION.md` - Complete documentation

**Files Modified**:
- `internal/services/storage_service.go` - Uses real client

**Dependencies Added**:
- `github.com/minio/minio-go/v7` - S3/MinIO client
- `github.com/disintegration/imaging` - Image processing

**See**: `STORAGE_IMPLEMENTATION.md` for complete documentation

---

### 2. ~~Missing Storage Package~~ ✅ **IMPLEMENTED**

**Status**: ✅ **FULLY IMPLEMENTED**

**What Was Done**:
- ✅ Created `pkg/storage/` package
- ✅ MinIO client with bucket management
- ✅ Image processing (resize, crop, compress, JPEG/PNG)
- ✅ Thumbnail generation capabilities
- ✅ Presigned URL generation
- ✅ Advanced image manipulation (sharpen, blur, rotate, flip)

---

## 🚨 Critical Missing Features

### ~~1. S3/MinIO Storage~~ ✅ DONE
### ~~2. Storage Package~~ ✅ DONE

---

## ⚠️ Important Missing Features

### 3. Swagger/OpenAPI Documentation

**Status**: ⚠️ **NOT GENERATED**

**Current State**:
- Swagger comments exist in handlers
- `docs/` directory doesn't exist
- Swagger UI not set up

**What's Needed**:
```bash
# Generate Swagger docs
make swagger

# Or manually
swag init -g cmd/server/main.go -o ./docs
```

**Files to Create**:
- `docs/docs.go`
- `docs/swagger.json`
- `docs/swagger.yaml`

**Impact**: 🟡 MEDIUM - Helpful for frontend integration

**Estimated Effort**: 1-2 hours

---

### 4. Production Docker Compose Configuration

**Status**: ❌ **NOT CREATED**

**Current State**:
- `docker-compose.yml` exists for development
- No `docker-compose.prod.yml` for production

**What's Needed**:
- Create `docker-compose.prod.yml`
- Configure with production settings
- Add nginx reverse proxy
- Add SSL/TLS certificates
- Add health checks
- Add restart policies
- Add resource limits

**Impact**: 🟡 MEDIUM - Documented in DEPLOYMENT.md but file doesn't exist

**Estimated Effort**: 2-3 hours

---

### 5. Air Configuration for Hot Reload

**Status**: ❌ **NOT CREATED**

**Current State**:
- Makefile has `make dev` command
- `.air.toml` configuration file missing

**What's Needed**:
- Create `.air.toml` configuration
- Configure file watching patterns
- Configure build and run commands

**Impact**: 🟢 LOW - Developer convenience only

**Estimated Effort**: 30 minutes

---

## 📝 Code TODO Items

### 6. Admin Role Middleware

**Status**: ⚠️ **NOT IMPLEMENTED**

**Location**: `cmd/server/main.go:339`

```go
// TODO: Add admin role middleware when implemented
adminCategories := v1.Group("/admin/categories")
adminCategories.Use(authMiddleware.RequireAuth())
```

**What's Needed**:
1. Add `role` field to users table (if not exists)
2. Create `RequireAdmin()` middleware
3. Check user role in middleware
4. Apply to admin routes

**Impact**: 🟡 MEDIUM - Admin routes currently accessible by any authenticated user

**Estimated Effort**: 2-3 hours

---

### 7. Session Verification in Auth Middleware

**Status**: ⚠️ **INCOMPLETE**

**Location**: `internal/middleware/auth.go`

```go
// TODO: Implement session verification
```

**What's Needed**:
1. Verify session exists in `user_sessions` table
2. Check session is not revoked
3. Check device fingerprint if available
4. Update last_activity timestamp

**Impact**: 🟡 MEDIUM - Sessions can't be revoked currently

**Estimated Effort**: 2-3 hours

---

### 8. WebSocket Origin Checking

**Status**: ⚠️ **INCOMPLETE**

**Location**: `internal/handlers/chat_handler.go`

```go
// TODO: Implement proper origin checking based on CORS config
```

**What's Needed**:
1. Read allowed origins from CORS config
2. Validate WebSocket upgrade origin
3. Reject unauthorized origins

**Impact**: 🟡 MEDIUM - Security issue for WebSocket connections

**Estimated Effort**: 1 hour

---

### 9. MFA Password Verification

**Status**: ⚠️ **INCOMPLETE**

**Location**: `internal/handlers/mfa_handler.go`

```go
// TODO: Verify password before disabling
```

**What's Needed**:
1. Require password confirmation before disabling MFA
2. Add password field to disable MFA request
3. Verify password before disabling

**Impact**: 🟡 MEDIUM - Security issue, MFA can be disabled without password

**Estimated Effort**: 1 hour

---

### 10. Profile Stats Population

**Status**: ⚠️ **INCOMPLETE**

**Location**: `internal/services/profile_service.go`

```go
// TODO: Populate stats (followers, following, posts count)
// TODO: Populate relationship status if viewerID is provided
```

**What's Needed**:
1. Query followers/following count
2. Query posts count
3. Query relationship status between viewer and profile user
4. Add to profile response

**Impact**: 🟢 LOW - Stats already available via separate endpoints

**Estimated Effort**: 2-3 hours

---

### 11. Location Update Handling

**Status**: ⚠️ **INCOMPLETE**

**Location**: `internal/services/profile_service.go`

```go
// TODO: Handle location update (Latitude/Longitude -> pgtype.Point)
```

**What's Needed**:
1. Convert latitude/longitude to PostGIS POINT
2. Update address_location field
3. Optionally geocode to get address text

**Impact**: 🟢 LOW - Location filtering works, just profile location update missing

**Estimated Effort**: 1-2 hours

---

### 12. Current Session Exclusion in Logout All

**Status**: ⚠️ **INCOMPLETE**

**Location**: `internal/services/auth_service.go`

```go
// TODO: Pass current session ID to exclude it
```

**What's Needed**:
1. Store session ID in JWT claims
2. Extract session ID in logout all
3. Exclude current session from deletion

**Impact**: 🟢 LOW - "Logout all except current" feature

**Estimated Effort**: 1-2 hours

---

### 13. Photo Metadata from Storage

**Status**: ⚠️ **INCOMPLETE**

**Location**:
- `internal/services/post_service.go`
- `internal/services/comment_service.go`

```go
// TODO: Get photo metadata (width, height, size) from storage service when available
```

**What's Needed**:
1. Once storage service is implemented, fetch actual photo metadata
2. Populate width, height, size fields
3. Generate thumbnails

**Impact**: 🟢 LOW - Photo URLs work, just missing metadata

**Estimated Effort**: Depends on storage implementation

---

## 🧪 Testing Gaps

### 14. Limited Test Coverage

**Status**: ⚠️ **INCOMPLETE**

**Current Tests**:
- ✅ `config/config_test.go`
- ✅ `internal/utils/response_test.go`
- ✅ `internal/utils/validator_test.go`
- ✅ `internal/services/password_service_test.go`
- ✅ `internal/services/jwt_service_test.go`

**Missing Tests**:
- ❌ Handler tests
- ❌ Repository tests
- ❌ Service tests (most services)
- ❌ Integration tests
- ❌ E2E tests
- ❌ Load/performance tests

**What's Needed**:
1. Unit tests for all handlers
2. Unit tests for all services
3. Unit tests for all repositories
4. Integration tests for API endpoints
5. E2E tests for critical flows
6. Load tests with realistic data

**Impact**: 🟡 MEDIUM - No automated testing safety net

**Estimated Effort**: 40-60 hours for comprehensive coverage

---

### 15. No Integration Tests

**Status**: ❌ **NOT IMPLEMENTED**

**What's Needed**:
1. Create test database setup
2. Write integration tests for API endpoints
3. Test database transactions
4. Test Redis caching
5. Test WebSocket connections

**Impact**: 🟡 MEDIUM - Can't verify end-to-end functionality

**Estimated Effort**: 20-30 hours

---

## 🔍 Optional Enhancements

### 16. Location Service Package

**Status**: ⚠️ **STUB IMPLEMENTATION**

**Current State**:
- `pkg/location/` directory doesn't exist
- Geocoding referenced in config but not implemented
- Distance calculations done in SQL

**What's Needed**:
1. Geocoding service (Google Maps API integration)
2. Reverse geocoding
3. Address parsing
4. Distance calculation utilities
5. Location validation

**Impact**: 🟢 LOW - PostGIS handles most location features

**Estimated Effort**: 4-6 hours

---

### 17. Email Templates

**Status**: ⚠️ **BASIC IMPLEMENTATION**

**Current State**:
- Email service exists
- Simple text emails
- No HTML templates

**What's Needed**:
1. HTML email templates
2. Template variables (name, verification code, etc.)
3. Multi-language support
4. Email preview/testing

**Impact**: 🟢 LOW - Functional but not polished

**Estimated Effort**: 4-6 hours

---

### 18. Rate Limiting Customization

**Status**: ✅ **IMPLEMENTED BUT BASIC**

**Current State**:
- Rate limiting works
- Fixed limits per endpoint type
- Redis-based

**Potential Enhancements**:
1. Per-user rate limits
2. Dynamic rate limits based on user tier
3. Rate limit headers (X-RateLimit-Remaining)
4. Rate limit bypass for admins
5. IP-based rate limiting

**Impact**: 🟢 LOW - Current implementation sufficient

**Estimated Effort**: 4-8 hours

---

### 19. Caching Layer

**Status**: ⚠️ **NOT IMPLEMENTED**

**Current State**:
- Redis available but only used for rate limiting and token storage
- No query result caching
- No frequently-accessed data caching

**What's Needed**:
1. Cache frequently accessed profiles
2. Cache feed posts (short TTL)
3. Cache business profiles
4. Cache category lists
5. Cache notification counts
6. Implement cache invalidation strategy

**Impact**: 🟢 LOW - Performance optimization

**Estimated Effort**: 8-12 hours

---

### 20. Observability Enhancements

**Status**: ✅ **BASIC IMPLEMENTATION**

**Current State**:
- Structured logging works
- Basic health checks
- No distributed tracing
- No APM

**Potential Enhancements**:
1. Distributed tracing (Jaeger/Zipkin)
2. Custom Prometheus metrics
3. APM integration (New Relic, Datadog)
4. Error tracking (Sentry already in config)
5. Request/response logging middleware

**Impact**: 🟢 LOW - Nice to have for production

**Estimated Effort**: 8-12 hours

---

## 📊 Summary

### By Priority

#### 🔴 Critical (Must Have Before Production)
1. **S3/MinIO Storage Integration** - Image uploads don't work
2. **Storage Package Implementation** - Required for image processing
3. **Admin Role Middleware** - Security issue
4. **Session Verification** - Security issue
5. **WebSocket Origin Checking** - Security issue
6. **MFA Password Verification** - Security issue

**Total Critical Estimated Effort**: 20-30 hours

#### 🟡 Important (Should Have)
1. **Swagger Documentation** - Developer experience
2. **Docker Compose Production Config** - Deployment ease
3. **Comprehensive Test Suite** - Quality assurance
4. **Integration Tests** - Functionality verification

**Total Important Estimated Effort**: 60-90 hours

#### 🟢 Optional (Nice to Have)
1. **Air Configuration** - Developer convenience
2. **Profile Stats Population** - Feature completeness
3. **Location Service** - Enhanced location features
4. **Email Templates** - Polish
5. **Caching Layer** - Performance
6. **Enhanced Observability** - Operations

**Total Optional Estimated Effort**: 40-60 hours

---

## 🎯 Recommended Action Plan

### Phase 1: Critical Security & Functionality (Week 1)
1. ✅ Implement S3/MinIO storage integration (Day 1-2)
2. ✅ Create storage package with image processing (Day 2-3)
3. ✅ Implement admin role middleware (Day 3)
4. ✅ Add session verification (Day 4)
5. ✅ Fix WebSocket origin checking (Day 4)
6. ✅ Add MFA password verification (Day 5)

### Phase 2: Developer Experience (Week 2)
1. ✅ Generate Swagger documentation (Day 1)
2. ✅ Create docker-compose.prod.yml (Day 1)
3. ✅ Create .air.toml configuration (Day 1)
4. ✅ Write integration tests for critical endpoints (Day 2-5)

### Phase 3: Testing (Week 3-4)
1. ✅ Write handler tests (Week 3)
2. ✅ Write service tests (Week 3-4)
3. ✅ Write repository tests (Week 4)
4. ✅ E2E tests for critical flows (Week 4)

### Phase 4: Polish & Optimization (Week 5+)
1. ✅ Implement caching layer
2. ✅ Add location service
3. ✅ Create HTML email templates
4. ✅ Enhanced observability
5. ✅ Performance testing and optimization

---

## 🔧 Quick Fixes (Can Do Now)

These can be fixed in < 30 minutes each:

1. Create `.air.toml` configuration
2. Fix WebSocket origin checking
3. Add MFA password verification requirement
4. Generate Swagger documentation
5. Create `docker-compose.prod.yml`

---

## 📝 Notes

- Most TODOs are small enhancements or polish items
- Core functionality is implemented and working
- Main blocker for production is S3/MinIO storage integration
- Security TODOs should be addressed before production
- Testing should be prioritized for confidence in production deployment

---

**Last Updated**: October 16, 2025
**Reviewers**: Development Team
**Status**: Ready for prioritization and implementation
