# Hamsaya Platform Implementation Guides

This document contains comprehensive implementation guides for fixing critical issues, hardening security, and deploying the Hamsaya platform.

**Last Updated:** 2025-01-24

## Table of Contents

1. [Critical Mobile Fixes](#critical-mobile-fixes)
2. [Critical Backend Fixes](#critical-backend-fixes)
3. [Critical Dashboard Fixes](#critical-dashboard-fixes)
4. [Test Framework Setup](#test-framework-setup)
5. [Security Hardening](#security-hardening)
6. [Database Migrations](#database-migrations)
7. [Deployment Guide](#deployment-guide)

---

## Critical Mobile Fixes

### 1. FCM Notification Implementation

**Issue:** FCM token registration not working - commented out in notification_service.dart

**Fix Location:** `lib/src/helper/services/notification_service.dart`

**Step 1: Uncomment Token Handling**

Find lines 19-50 and uncomment:
```dart
_setupTokenHandling();
_setupNotificationChannels();
_setupFirebaseListeners();
```

**Step 2: Add Token Registration Method**

Add this method to NotificationService:

```dart
Future<void> _registerTokenWithBackend(String token) async {
  try {
    final apiClient = ref.read(apiClientProvider);
    await apiClient.post(
      '/notifications/fcm-token',
      data: {'token': token},
    );
    debugPrint('FCM token registered with backend: $token');
  } catch (e) {
    debugPrint('Failed to register FCM token: $e');
  }
}
```

**Step 3: Update auth_provider_dio.dart**

In the login success handler, add FCM token registration:

```dart
// After successful login
final notificationService = ref.read(notificationServiceProvider);
await notificationService.registerFCMToken();
```

**Verification:**
1. Login to the app
2. Check backend logs for FCM token registration
3. Send a test notification from Firebase Console
4. Verify notification appears on device

---

### 2. Business Profile Image Upload Fix

**Issue:** Upload methods return success but don't actually upload to backend

**Fix Location:** `lib/src/featured/business/provider/business_profile_uploader_provider.dart`

**Current Code (lines 20-30):**
```dart
Future<void> uploadAvatar(String businessId, File imageFile) async {
  state = const AsyncValue.loading();
  state = AsyncValue.data(imageFile.path); // Wrong!
}
```

**Fixed Code:**
```dart
Future<void> uploadAvatar(String businessId, File imageFile) async {
  state = const AsyncValue.loading();

  try {
    final apiClient = ref.read(apiClientProvider);

    // Create multipart file
    final fileName = imageFile.path.split('/').last;
    final multipartFile = await MultipartFile.fromFile(
      imageFile.path,
      filename: fileName,
    );

    // Create form data
    final formData = FormData.fromMap({
      'avatar': multipartFile,
    });

    // Upload to backend
    await apiClient.post(
      '/businesses/$businessId/avatar',
      data: formData,
    );

    // Invalidate cache to refetch
    ref.invalidate(businessProfileProvider(businessId));

    state = AsyncValue.data(imageFile.path);
  } catch (e, stack) {
    state = AsyncValue.error(e, stack);
  }
}
```

**Similarly fix uploadCover:**
```dart
Future<void> uploadCover(String businessId, File imageFile) async {
  // Same pattern, use endpoint: '/businesses/$businessId/cover'
}
```

---

## Critical Backend Fixes

### 1. Admin Route Protection

**Issue:** Admin routes not protected - any authenticated user can access

**Fix Location:** `cmd/server/main.go`

**Step 1: Verify Middleware Exists**

Check `internal/middleware/auth.go` has RequireAdmin method.

**Step 2: Apply Middleware**

In your router setup:

```go
// Admin routes group
admin := v1.Group("/admin")
admin.Use(authMiddleware.RequireAuth())      // First: Require authentication
admin.Use(authMiddleware.RequireAdmin())     // Second: Require admin role

// All admin routes now protected
admin.GET("/users", adminHandler.GetUsers)
admin.DELETE("/users/:id", adminHandler.DeleteUser)
admin.GET("/posts", adminHandler.GetAllPosts)
admin.DELETE("/posts/:id", adminHandler.DeletePost)
admin.GET("/reports", adminHandler.GetReports)
admin.PUT("/reports/:id/resolve", adminHandler.ResolveReport)
```

**Step 3: Test Protection**

```bash
# Should fail (403 Forbidden)
curl -H "Authorization: Bearer <regular-user-token>" \
  http://localhost:8000/api/v1/admin/users

# Should succeed (200 OK)
curl -H "Authorization: Bearer <admin-token>" \
  http://localhost:8000/api/v1/admin/users
```

---

### 2. CORS Security Fix

**Issue:** CORS allows wildcard (*) in production

**Fix Location:** `internal/middleware/cors.go`

**Updated Code:**

```go
func NewCORSMiddleware(config CORSConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")

        // CRITICAL: Reject wildcard in production
        if config.Environment == "production" && origin == "*" {
            c.AbortWithStatusJSON(403, gin.H{
                "error": "Wildcard origin not allowed in production",
            })
            return
        }

        // Check if origin is allowed
        allowed := false
        for _, allowedOrigin := range config.AllowedOrigins {
            if allowedOrigin == origin {
                allowed = true
                c.Header("Access-Control-Allow-Origin", origin)
                break
            }
        }

        if !allowed && config.Environment == "production" {
            c.AbortWithStatusJSON(403, gin.H{
                "error": fmt.Sprintf("Origin %s not allowed", origin),
            })
            return
        }

        // ... rest of CORS headers
        c.Next()
    }
}
```

**Update .env:**
```bash
ENVIRONMENT=production
CORS_ALLOWED_ORIGINS=https://hamsaya.com,https://admin.hamsaya.com
```

---

### 3. SQL Injection Fix - Admin Repository

**Issue:** ILIKE parameter mismatch bug

**Fix Location:** `internal/repositories/admin_repository.go` (lines 864-866)

**BEFORE (VULNERABLE):**
```go
query += fmt.Sprintf(" WHERE (email ILIKE $%d OR first_name ILIKE $%d OR last_name ILIKE $%d)",
    paramCount, paramCount, paramCount)  // BUG: Same parameter 3 times
params = append(params, "%"+search+"%")  // Only one value!
```

**AFTER (FIXED):**
```go
query += fmt.Sprintf(" WHERE (email ILIKE $%d OR first_name ILIKE $%d OR last_name ILIKE $%d)",
    paramCount, paramCount+1, paramCount+2)  // FIXED: Unique parameters
params = append(params, "%"+search+"%", "%"+search+"%", "%"+search+"%")  // Three values
paramCount += 3
```

---

## Critical Dashboard Fixes

### 1. Secure Token Storage

**Issue:** Tokens stored in localStorage (XSS vulnerable)

**Fix Location:** `src/lib/api-client.ts`

**Already fixed in previous implementation guides - tokens now stored in httpOnly cookies.**

---

### 2. Input Sanitization

**Create:** `src/lib/sanitize.ts`

```typescript
import DOMPurify from 'dompurify';

export function sanitizeHTML(dirty: string): string {
  return DOMPurify.sanitize(dirty, {
    ALLOWED_TAGS: ['b', 'i', 'em', 'strong', 'a', 'p', 'br'],
    ALLOWED_ATTR: ['href', 'target'],
  });
}

export function sanitizeSearchInput(input: string): string {
  return input
    .replace(/[;'"\\]/g, '')
    .replace(/\b(union|select|insert|update|delete|drop)\b/gi, '')
    .trim()
    .substring(0, 100);
}
```

**Install dependency:**
```bash
npm install dompurify @types/dompurify
```

---

## Test Framework Setup

### Backend (Go)

**Install Dependencies:**
```bash
go get github.com/stretchr/testify/assert
go get github.com/stretchr/testify/mock
go get github.com/DATA-DOG/go-sqlmock
```

**Example Test:** `internal/services/auth_service_test.go`

```go
func TestAuthService_Login(t *testing.T) {
    userRepo := new(mocks.MockUserRepository)
    authService := NewAuthService(userRepo, nil, nil, nil, nil)

    // Test successful login
    t.Run("successful login", func(t *testing.T) {
        hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
        user := &models.User{
            ID:           "user-id",
            Email:        "test@example.com",
            PasswordHash: string(hashedPassword),
            IsActive:     true,
        }

        userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(user, nil)
        userRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

        tokens, err := authService.Login(context.Background(), "test@example.com", "password123", "device", "127.0.0.1")

        assert.NoError(t, err)
        assert.NotNil(t, tokens)
        assert.NotEmpty(t, tokens.AccessToken)
    })
}
```

**Run Tests:**
```bash
make test
make test-coverage
```

---

### Mobile (Flutter)

**Add to pubspec.yaml:**
```yaml
dev_dependencies:
  mockito: ^5.4.4
  build_runner: ^2.4.8
```

**Example Test:** `test/unit/providers/auth_provider_test.dart`

```dart
void main() {
  late MockAuthRepositoryDio mockAuthRepo;
  late ProviderContainer container;

  setUp(() {
    mockAuthRepo = MockAuthRepositoryDio();
    container = ProviderContainer(
      overrides: [
        authRepositoryProvider.overrideWithValue(mockAuthRepo),
      ],
    );
  });

  test('login success updates state to authenticated', () async {
    final loginRequest = LoginRequest(
      email: 'test@example.com',
      password: 'password123',
    );

    when(mockAuthRepo.login(loginRequest))
        .thenAnswer((_) async => Future.value());

    await container.read(authProvider.notifier).login(loginRequest);

    final state = container.read(authProvider);
    expect(state, isA<Authenticated>());
  });
}
```

---

## Security Hardening

### 1. Environment Variables Validation

**Create:** `scripts/validate_secrets.sh`

```bash
#!/bin/bash
set -e

REQUIRED_SECRETS=(
    "DATABASE_URL"
    "REDIS_URL"
    "JWT_SECRET"
    "JWT_REFRESH_SECRET"
)

MISSING_SECRETS=()

for SECRET in "${REQUIRED_SECRETS[@]}"; do
    VALUE="${!SECRET}"
    if [ -z "$VALUE" ]; then
        MISSING_SECRETS+=("$SECRET")
    elif [ ${#VALUE} -lt 32 ]; then
        echo "⚠️  $SECRET is weak (length: ${#VALUE}, required: 32+)"
        exit 1
    fi
done

if [ ${#MISSING_SECRETS[@]} -gt 0 ]; then
    echo "❌ Missing required secrets:"
    printf '  - %s\n' "${MISSING_SECRETS[@]}"
    exit 1
fi

echo "✅ All secrets validated"
```

---

### 2. Rate Limiting Enhancement

**Create:** `internal/middleware/rate_limit_enhanced.go`

```go
type RateLimiter struct {
    redis  *redis.Client
    logger *zap.Logger
}

func (rl *RateLimiter) Limit(config RateLimitConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := config.KeyFunc(c)
        now := time.Now().Unix()

        // Sliding window counter using Redis sorted set
        count, err := rl.redis.ZCard(context.Background(), key).Result()

        if count >= int64(config.Requests) {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "Rate limit exceeded",
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

**Apply to sensitive endpoints:**
```go
// Auth endpoints - 5 requests per 15 minutes
authGroup.Use(rateLimiter.Limit(RateLimitConfig{
    Requests: 5,
    Window:   15 * time.Minute,
    KeyFunc:  KeyByIP,
}))

// Post creation - 20 per hour
postsGroup.POST("", rateLimiter.Limit(RateLimitConfig{
    Requests: 20,
    Window:   1 * time.Hour,
    KeyFunc:  KeyByUserID,
}))
```

---

## Database Migrations

All migration files have been created in `migrations/` directory:

1. **20250124120000_add_is_active_indexes** - Performance indexes
2. **20250124120100_add_composite_indexes** - Composite indexes for queries
3. **20250124120200_add_post_type_constraints** - Type-specific validation
4. **20250124120300_add_unique_constraints** - Data integrity
5. **20250124120400_optimize_postgis** - Spatial query optimization
6. **20250124120500_fix_conversations** - Conversation ordering
7. **20250124120600_add_foreign_key_constraints** - CASCADE rules

**Run migrations:**
```bash
make migrate-up
```

**Verify:**
```bash
make migrate-status
```

See `migrations/README.md` for detailed migration guide.

---

## Deployment Guide

### Pre-Deployment Checklist

- [ ] All tests passing (80%+ coverage)
- [ ] Security audit completed
- [ ] Secrets validated
- [ ] Database backup created
- [ ] Rollback plan documented
- [ ] Team notified

### Infrastructure Requirements

**Application Servers:**
- CPU: 4 cores
- RAM: 8GB
- Disk: 100GB SSD
- Quantity: 2+ (HA)

**Database Server:**
- CPU: 8 cores
- RAM: 16GB
- Disk: 500GB SSD
- Quantity: 1 primary + 1 replica

### Deployment Steps

**1. Deploy Backend**
```bash
./scripts/deploy.sh production
```

**2. Run Migrations**
```bash
ssh backend-server
cd /opt/hamsaya/backend
./hamsaya-backend migrate up
```

**3. Deploy Dashboard**
```bash
cd hamsaya-dashboard
npm run build
# Upload dist/ to CDN
```

**4. Verify**
```bash
# Health check
curl https://api.hamsaya.com/health

# Test authentication
curl -X POST https://api.hamsaya.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"test123"}'
```

### Post-Deployment Monitoring

Monitor for 24 hours:
- Error rate < 1%
- Response time p95 < 500ms
- Database CPU < 60%
- No critical errors in logs

---

## Additional Resources

- **SECURITY_CHECKLIST.md** - Comprehensive security checklist
- **DEPLOYMENT_CHECKLIST.md** - Step-by-step deployment guide
- **migrations/README.md** - Database migration guide
- **scripts/** - Deployment and utility scripts

---

## Support

For issues or questions, contact the development team.
