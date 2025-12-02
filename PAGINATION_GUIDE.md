# Backend Pagination Guide

This guide explains how to use the updated pagination system in the Hamsaya backend.

## Overview

The pagination system has been updated to match the frontend structure:

```json
{
  "success": true,
  "message": "Data retrieved successfully",
  "data": [...],
  "meta": {
    "currentPage": 1,
    "itemsPerPage": 20,
    "totalItems": 313,
    "totalPages": 16,
    "filters": {},
    "sorts": {}
  }
}
```

## Pagination Struct

Located in `internal/utils/response.go`:

```go
type Pagination struct {
    CurrentPage  int                    `json:"currentPage"`
    ItemsPerPage int                    `json:"itemsPerPage"`
    TotalItems   int64                  `json:"totalItems"`
    TotalPages   int                    `json:"totalPages"`
    Filters      map[string]interface{} `json:"filters,omitempty"`
    Sorts        map[string]interface{} `json:"sorts,omitempty"`
}
```

## Basic Usage

### Simple Pagination

Use `SendPaginated` for basic paginated responses:

```go
func (h *PostHandler) GetPosts(c *gin.Context) {
    // Parse pagination parameters
    page := 1
    limit := 20
    if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
        page = p
    }
    if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
        limit = l
    }

    // Calculate offset
    offset := (page - 1) * limit

    // Fetch data from repository
    posts, totalCount, err := h.postRepo.List(ctx, limit, offset)
    if err != nil {
        utils.SendInternalServerError(c, "Failed to fetch posts", err)
        return
    }

    // Send paginated response
    utils.SendPaginated(c, posts, page, limit, totalCount)
}
```

**Response:**
```json
{
  "success": true,
  "data": [...],
  "meta": {
    "currentPage": 1,
    "itemsPerPage": 20,
    "totalItems": 313,
    "totalPages": 16,
    "filters": null,
    "sorts": null
  }
}
```

### Pagination with Filters and Sorts

Use `SendPaginatedWithFilters` to include filter and sort information:

```go
func (h *PostHandler) GetPosts(c *gin.Context) {
    // Parse parameters
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

    // Parse filters
    postType := c.Query("type")
    categoryID := c.Query("category_id")

    // Parse sorts
    sortBy := c.DefaultQuery("sort_by", "created_at")
    sortOrder := c.DefaultQuery("sort_order", "desc")

    // Build filters map
    filters := make(map[string]interface{})
    if postType != "" {
        filters["type"] = postType
    }
    if categoryID != "" {
        filters["category_id"] = categoryID
    }

    // Build sorts map
    sorts := map[string]interface{}{
        "sort_by":    sortBy,
        "sort_order": sortOrder,
    }

    // Fetch data
    posts, totalCount, err := h.postRepo.ListWithFilters(
        ctx,
        limit,
        (page-1)*limit,
        postType,
        categoryID,
        sortBy,
        sortOrder,
    )
    if err != nil {
        utils.SendInternalServerError(c, "Failed to fetch posts", err)
        return
    }

    // Send paginated response with filters
    utils.SendPaginatedWithFilters(c, posts, page, limit, totalCount, filters, sorts)
}
```

**Response:**
```json
{
  "success": true,
  "data": [...],
  "meta": {
    "currentPage": 1,
    "itemsPerPage": 20,
    "totalItems": 313,
    "totalPages": 16,
    "filters": {
      "type": "FEED",
      "category_id": "123"
    },
    "sorts": {
      "sort_by": "created_at",
      "sort_order": "desc"
    }
  }
}
```

## Repository Layer

Your repository should return both data and total count:

```go
func (r *PostRepository) List(ctx context.Context, limit, offset int) ([]*models.Post, int64, error) {
    var posts []*models.Post
    var totalCount int64

    // Get total count
    err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL").Scan(&totalCount)
    if err != nil {
        return nil, 0, err
    }

    // Get paginated data
    query := `
        SELECT id, title, content, created_at, updated_at
        FROM posts
        WHERE deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `
    rows, err := r.db.Query(ctx, query, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    for rows.Next() {
        var post models.Post
        err := rows.Scan(&post.ID, &post.Title, &post.Content, &post.CreatedAt, &post.UpdatedAt)
        if err != nil {
            return nil, 0, err
        }
        posts = append(posts, &post)
    }

    return posts, totalCount, nil
}
```

## Handler Best Practices

### 1. Parse Pagination Parameters

```go
// Set defaults and validate
page := 1
limit := 20

if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
    page = p
}
if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 100 {
    limit = l
}

offset := (page - 1) * limit
```

### 2. Handle Empty Results

```go
posts, totalCount, err := h.postRepo.List(ctx, limit, offset)
if err != nil {
    utils.SendInternalServerError(c, "Failed to fetch posts", err)
    return
}

// Empty array is valid - frontend handles this
if posts == nil {
    posts = []*models.Post{} // Ensure empty array, not null
}

utils.SendPaginated(c, posts, page, limit, totalCount)
```

### 3. Limit Maximum Page Size

```go
const MaxPageSize = 100

limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
if limit > MaxPageSize {
    limit = MaxPageSize
}
```

## Common Query Parameters

Standardize these across all paginated endpoints:

- `page` - Page number (1-indexed, default: 1)
- `limit` - Items per page (default: 20, max: 100)
- `sort_by` - Field to sort by (default: "created_at")
- `sort_order` - Sort direction: "asc" or "desc" (default: "desc")

**Example URL:**
```
GET /api/v1/posts?page=2&limit=20&type=FEED&sort_by=created_at&sort_order=desc
```

## Testing Pagination

```bash
# First page
curl "http://localhost:8080/api/v1/posts?page=1&limit=10"

# Second page
curl "http://localhost:8080/api/v1/posts?page=2&limit=10"

# With filters
curl "http://localhost:8080/api/v1/posts?page=1&limit=10&type=FEED&category_id=123"

# Check total pages calculation
# If totalItems = 313 and limit = 20, then totalPages should be 16
# (313 / 20 = 15.65, rounded up = 16)
```

## Migration Notes

**Old format** (deprecated):
```json
{
  "meta": {
    "page": 1,
    "limit": 20,
    "total_pages": 16,
    "total_count": 313
  }
}
```

**New format** (current):
```json
{
  "meta": {
    "currentPage": 1,
    "itemsPerPage": 20,
    "totalPages": 16,
    "totalItems": 313,
    "filters": {},
    "sorts": {}
  }
}
```

All existing `SendPaginated` calls will continue to work and now use the new format.

## Complete Example

```go
// Handler
func (h *PostHandler) GetFeed(c *gin.Context) {
    // Parse pagination
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
    if limit > 100 {
        limit = 100
    }

    // Parse filters
    postType := c.Query("type")
    categoryID := c.Query("category_id")

    // Parse sorts
    sortBy := c.DefaultQuery("sort_by", "created_at")
    sortOrder := c.DefaultQuery("sort_order", "desc")

    // Calculate offset
    offset := (page - 1) * limit

    // Fetch data
    posts, totalCount, err := h.postService.GetFeed(
        c.Request.Context(),
        limit,
        offset,
        postType,
        categoryID,
        sortBy,
        sortOrder,
    )
    if err != nil {
        utils.SendInternalServerError(c, "Failed to fetch feed", err)
        return
    }

    // Build response metadata
    filters := make(map[string]interface{})
    if postType != "" {
        filters["type"] = postType
    }
    if categoryID != "" {
        filters["category_id"] = categoryID
    }

    sorts := map[string]interface{}{
        "sort_by":    sortBy,
        "sort_order": sortOrder,
    }

    // Send response
    utils.SendPaginatedWithFilters(c, posts, page, limit, totalCount, filters, sorts)
}
```

## Troubleshooting

### Wrong total pages calculation

If `totalPages` is incorrect, check:
1. Repository is returning correct `totalCount`
2. Division is handled correctly (rounds up)

```go
totalPages := int(totalCount) / limit
if int(totalCount)%limit != 0 {
    totalPages++ // Round up
}
```

### null vs empty array

Always return empty array `[]` instead of `null`:

```go
if posts == nil {
    posts = []*models.Post{}
}
```

### Performance with large offsets

For very large datasets, consider using cursor-based pagination instead of offset-based.
