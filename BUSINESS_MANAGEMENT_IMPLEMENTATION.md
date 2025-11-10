# Business Management Implementation for Admin Dashboard

This document describes the complete implementation of the Business Management feature for the admin dashboard.

## Overview

The Business Management feature allows administrators to:
- View a paginated list of all businesses with filtering and search capabilities
- Update the active/inactive status of any business
- Search businesses by name, license number, email, phone number, province, or district
- Filter businesses by active/inactive status

## API Endpoints

### 1. List Businesses
**Endpoint:** `GET /api/v1/admin/businesses`

**Query Parameters:**
- `search` (optional): Search term for name, license_no, email, phone_number, province, district, or owner email
- `status` (optional): Filter by status - `true` for active, `false` for inactive
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 20, max: 100)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "user_id": "uuid",
      "owner_email": "owner@example.com",
      "owner_name": "John Doe",
      "name": "Business Name",
      "license_no": "LIC-12345",
      "email": "business@example.com",
      "phone_number": "+1234567890",
      "province": "Province",
      "district": "District",
      "status": true,
      "total_views": 1000,
      "total_follow": 50,
      "total_posts": 25,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-15T12:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total_count": 100,
    "total_pages": 5
  }
}
```

### 2. Update Business Status
**Endpoint:** `PUT /api/v1/admin/businesses/:id/status`

**Path Parameters:**
- `id`: Business ID (UUID)

**Request Body:**
```json
{
  "status": true
}
```

**Response:**
```json
{
  "success": true,
  "message": "Business status updated successfully"
}
```

**Error Responses:**
- `400 Bad Request`: Invalid request body or validation failed
- `404 Not Found`: Business not found
- `500 Internal Server Error`: Database or server error

## Implementation Details

### 1. Repository Layer (`internal/repositories/admin_repository.go`)

#### Interface Methods
```go
ListBusinesses(ctx context.Context, search string, status *bool, page, limit int) ([]models.AdminBusinessListItem, int64, error)
UpdateBusinessStatus(ctx context.Context, businessID string, status bool) error
```

#### Key Features
- **ListBusinesses:**
  - Joins with `users` table to get owner email
  - Joins with `profiles` table to construct owner name (COALESCE for first_name || ' ' || last_name)
  - Left join with `posts` table to count total posts (WHERE business_id = bp.id AND deleted_at IS NULL)
  - Filters by search term using ILIKE on multiple fields
  - Filters by status (active/inactive) if provided
  - Pagination with OFFSET and LIMIT
  - Returns total count with same filters

- **UpdateBusinessStatus:**
  - Updates `business_profiles` table
  - Sets `status = $1` and `updated_at = NOW()`
  - Only updates non-deleted businesses (WHERE deleted_at IS NULL)
  - Returns error if no rows affected (business not found)

### 2. Service Layer (`internal/services/admin_service.go`)

#### Methods
```go
ListBusinesses(ctx context.Context, search string, status *bool, page, limit int) ([]models.AdminBusinessListItem, int64, error)
UpdateBusinessStatus(ctx context.Context, businessID string, status bool) error
```

#### Key Features
- Structured logging for all operations
- Logs input parameters on entry
- Logs results/errors on completion
- Delegates to repository layer
- No business logic transformations needed

### 3. Handler Layer (`internal/handlers/admin_handler.go`)

#### Methods
```go
ListBusinesses(c *gin.Context)
UpdateBusinessStatus(c *gin.Context)
```

#### Key Features
- **ListBusinesses:**
  - Parses query parameters (search, status, page, limit)
  - Validates parameters (page >= 1, limit 1-100)
  - Calls service layer
  - Returns paginated response with utils.SendPaginated

- **UpdateBusinessStatus:**
  - Gets businessID from URL parameter
  - Binds and validates UpdateBusinessStatusRequest
  - Calls service layer
  - Returns 404 if business not found
  - Returns 200 on success

### 4. Routes (`cmd/server/main.go`)

Added to the admin group (requires admin authentication):
```go
admin.GET("/businesses", adminHandler.ListBusinesses)
admin.PUT("/businesses/:id/status", adminHandler.UpdateBusinessStatus)
```

## Models

### AdminBusinessListItem
```go
type AdminBusinessListItem struct {
    ID          string  `json:"id"`
    UserID      string  `json:"user_id"`
    OwnerEmail  *string `json:"owner_email"`
    OwnerName   *string `json:"owner_name"`
    Name        string  `json:"name"`
    LicenseNo   *string `json:"license_no"`
    Email       *string `json:"email"`
    PhoneNumber *string `json:"phone_number"`
    Province    *string `json:"province"`
    District    *string `json:"district"`
    Status      bool    `json:"status"`
    TotalViews  int64   `json:"total_views"`
    TotalFollow int64   `json:"total_follow"`
    TotalPosts  int64   `json:"total_posts"`
    CreatedAt   string  `json:"created_at"`
    UpdatedAt   string  `json:"updated_at"`
}
```

### UpdateBusinessStatusRequest
```go
type UpdateBusinessStatusRequest struct {
    Status bool `json:"status" validate:"required"`
}
```

## SQL Queries

### List Businesses Query
```sql
SELECT
    bp.id,
    bp.user_id,
    u.email as owner_email,
    CASE
        WHEN p.first_name IS NOT NULL AND p.last_name IS NOT NULL
        THEN p.first_name || ' ' || p.last_name
        WHEN p.first_name IS NOT NULL THEN p.first_name
        WHEN p.last_name IS NOT NULL THEN p.last_name
        ELSE NULL
    END as owner_name,
    bp.name,
    bp.license_no,
    bp.email,
    bp.phone_number,
    bp.province,
    bp.district,
    bp.status,
    bp.total_views,
    bp.total_follow,
    COALESCE((
        SELECT COUNT(*)
        FROM posts
        WHERE business_id = bp.id AND deleted_at IS NULL
    ), 0) as total_posts,
    bp.created_at,
    bp.updated_at
FROM business_profiles bp
LEFT JOIN users u ON bp.user_id = u.id
LEFT JOIN profiles p ON u.id = p.id
WHERE bp.deleted_at IS NULL
    [AND bp.status = $1]
    [AND (bp.name ILIKE $2 OR bp.license_no ILIKE $2 OR bp.email ILIKE $2
          OR bp.phone_number ILIKE $2 OR bp.province ILIKE $2 OR bp.district ILIKE $2
          OR u.email ILIKE $2)]
ORDER BY bp.created_at DESC
LIMIT $N OFFSET $M
```

### Update Business Status Query
```sql
UPDATE business_profiles
SET status = $1, updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL
```

## Testing

### Manual Testing

1. **List all businesses:**
```bash
curl -X GET "http://localhost:8000/api/v1/admin/businesses" \
  -H "Authorization: Bearer <admin_token>"
```

2. **Search businesses:**
```bash
curl -X GET "http://localhost:8000/api/v1/admin/businesses?search=restaurant" \
  -H "Authorization: Bearer <admin_token>"
```

3. **Filter by status:**
```bash
curl -X GET "http://localhost:8000/api/v1/admin/businesses?status=true" \
  -H "Authorization: Bearer <admin_token>"
```

4. **Paginate results:**
```bash
curl -X GET "http://localhost:8000/api/v1/admin/businesses?page=2&limit=50" \
  -H "Authorization: Bearer <admin_token>"
```

5. **Update business status:**
```bash
curl -X PUT "http://localhost:8000/api/v1/admin/businesses/<business_id>/status" \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"status": false}'
```

## Authentication & Authorization

Both endpoints require:
1. Valid JWT token in `Authorization: Bearer <token>` header
2. User must have `admin` role (enforced by `authMiddleware.RequireAdmin()`)

## Error Handling

All errors follow the standard error response format:
```json
{
  "success": false,
  "message": "Human-readable error message",
  "error": "Technical error details"
}
```

Common error scenarios:
- Invalid query parameters (400)
- Missing or invalid authentication (401)
- Insufficient permissions (403)
- Business not found (404)
- Database errors (500)

## Logging

All operations are logged with structured logging:
- Entry: Parameters logged on method entry
- Success: Count and total logged on successful completion
- Error: Error details logged for debugging

Example log entries:
```
INFO: Listing businesses | search="restaurant" status=true page=1 limit=20
INFO: Businesses listed successfully | count=15 total=45
INFO: Updating business status | business_id="uuid" status=false
INFO: Business status updated successfully | business_id="uuid" status=false
```

## Pattern Consistency

This implementation follows the exact same pattern as existing admin features:
- **Users Management**: ListUsers, UpdateUserStatus
- **Posts Management**: ListPosts
- **Reports Management**: ListReports, UpdateReportStatus

All use:
- Repository interfaces for data access
- Service layer for business logic and logging
- Handler layer for HTTP request/response
- Standard pagination and filtering patterns
- Consistent error handling and response formats
