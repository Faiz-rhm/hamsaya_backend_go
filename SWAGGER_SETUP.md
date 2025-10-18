# Swagger Documentation Setup

**Status**: ⏸️ **Partially Complete** - Needs annotation fixes

---

## What Was Done

✅ Added Swagger metadata annotations to `cmd/server/main.go`
✅ Added Swagger imports (`gin-swagger`, `swaggo/files`)
✅ Added Swagger UI route at `/swagger/*any`
✅ Installed `swag` CLI tool

---

## Current Issue

The Swagger documentation generation fails due to parse errors in existing handler annotations:

### Known Issues:

1. **Missing Type Definitions**:
   ```
   ParseComment error: cannot find type definition: utils.ErrorResponse
   ParseComment error: cannot find type definition: models.FollowerResponse
   ```

2. **Incomplete Annotations**: Some handlers reference types that don't exist or aren't properly exported.

---

## How to Fix

### Option 1: Fix Annotations (Recommended)

1. **Add Missing Type Definitions**:
   - Create `utils.ErrorResponse` type if needed for error responses
   - Ensure all referenced types are properly defined and exported

2. **Update Problem Annotations**:
   - File: `internal/handlers/search_handler.go`
     Line containing: `// @Failure 400 {object} utils.ErrorResponse`

   - File: `internal/handlers/relationships_handler.go`
     Line containing: `// @Success 200 {object} utils.Response{data=[]models.FollowerResponse}`

3. **Generate Documentation**:
   ```bash
   swag init -g cmd/server/main.go --parseDependency --parseInternal -o docs
   ```

### Option 2: Use Generic Response Types (Quick Fix)

Replace specific type references with generic responses:

```go
// BEFORE:
// @Failure 400 {object} utils.ErrorResponse

// AFTER:
// @Failure 400 {object} utils.Response
```

---

## Generate Swagger Documentation

Once annotations are fixed:

```bash
# Install swag CLI (if not installed)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate documentation
swag init -g cmd/server/main.go -o docs

# Or with full parsing:
swag init -g cmd/server/main.go --parseDependency --parseInternal --parseDepth 1 -o docs
```

---

## Access Swagger UI

Once generated, the Swagger UI will be available at:

**URL**: `http://localhost:8080/swagger/index.html`

---

## Swagger Configuration

The following configuration is already in `cmd/server/main.go`:

```go
// @title Hamsaya Backend API
// @version 1.0
// @description A production-ready Go backend for a social media mobile application
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
```

---

## Dependencies Added

```go
github.com/swaggo/swag v1.16.6
github.com/swaggo/gin-swagger v1.6.1
github.com/swaggo/files v1.0.1
```

---

## Next Steps

1. Review and fix the swagger comment annotations in handlers
2. Run `swag init` to generate documentation
3. Access Swagger UI at `/swagger/index.html`
4. Test API documentation

---

**Note**: This is a non-critical feature. The API works without Swagger documentation, but it provides a nice interface for API testing and exploration.
