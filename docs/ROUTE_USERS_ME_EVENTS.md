# GET /api/v1/users/me/events

This endpoint is implemented in `internal/handlers/post_handler.go` as `PostHandler.GetMyEvents`.

## Register the route

Wherever you register your API routes (e.g. in your server bootstrap or `main.go`), add:

```go
// Example: if you have a group for user-related routes under v1
users := v1.Group("/users")
users.GET("/me/events", authMiddleware, postHandler.GetMyEvents)
```

So that the full path is `GET /api/v1/users/me/events`.

## Query parameters

| Param        | Type   | Required | Description                          |
|-------------|--------|----------|--------------------------------------|
| event_state | string | yes      | `going` or `interested`              |
| page        | int    | no       | Page number (0 or 1 = first page)    |
| limit       | int    | no       | Page size (default 20, max 100)      |

## Response

Same shape as other post list endpoints: `{ "data": [ ...PostResponse ] }`.
