# Hamsaya Backend API Documentation

**Version**: 1.0.0
**Base URL**: `https://api.hamsaya.com/api/v1`
**Authentication**: Bearer Token (JWT)

---

## Table of Contents

1. [Authentication](#authentication)
2. [Users & Profiles](#users--profiles)
3. [Posts](#posts)
4. [Comments](#comments)
5. [Business Profiles](#business-profiles)
6. [Search & Discovery](#search--discovery)
7. [Chat & Messaging](#chat--messaging)
8. [Notifications](#notifications)
9. [Error Handling](#error-handling)

---

## Authentication

### Register

```http
POST /api/v1/auth/register
```

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "first_name": "John",
  "last_name": "Doe"
}
```

**Response:** `201 Created`
```json
{
  "success": true,
  "message": "User registered successfully",
  "data": {
    "user": {
      "id": "uuid",
      "email": "user@example.com",
      "email_verified": false
    },
    "tokens": {
      "access_token": "eyJhbGc...",
      "refresh_token": "eyJhbGc...",
      "expires_in": 900
    }
  }
}
```

### Login

```http
POST /api/v1/auth/login
```

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "user": {
      "id": "uuid",
      "email": "user@example.com",
      "mfa_enabled": false
    },
    "tokens": {
      "access_token": "eyJhbGc...",
      "refresh_token": "eyJhbGc...",
      "expires_in": 900
    }
  }
}
```

### OAuth Authentication

#### Google OAuth
```http
POST /api/v1/auth/oauth/google
```

**Request Body:**
```json
{
  "id_token": "google_id_token_here"
}
```

#### Apple OAuth
```http
POST /api/v1/auth/oauth/apple
```

**Request Body:**
```json
{
  "id_token": "apple_id_token_here",
  "authorization_code": "apple_auth_code_here"
}
```

#### Facebook OAuth
```http
POST /api/v1/auth/oauth/facebook
```

**Request Body:**
```json
{
  "access_token": "facebook_access_token_here"
}
```

### Refresh Token

```http
POST /api/v1/auth/refresh
Authorization: Bearer {refresh_token}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "message": "Token refreshed successfully",
  "data": {
    "access_token": "eyJhbGc...",
    "refresh_token": "eyJhbGc...",
    "expires_in": 900
  }
}
```

---

## Users & Profiles

### Get My Profile

```http
GET /api/v1/users/me
Authorization: Bearer {access_token}
```

**Response:** `200 OK`
```json
{
  "success": true,
  "message": "Profile retrieved successfully",
  "data": {
    "id": "uuid",
    "first_name": "John",
    "last_name": "Doe",
    "full_name": "John Doe",
    "email": "user@example.com",
    "avatar": {
      "url": "https://cdn.example.com/avatar.webp",
      "width": 400,
      "height": 400
    },
    "cover": {
      "url": "https://cdn.example.com/cover.webp",
      "width": 1600,
      "height": 900
    },
    "about": "Software developer",
    "province": "California",
    "followers_count": 150,
    "following_count": 200,
    "posts_count": 45
  }
}
```

### Update Profile

```http
PUT /api/v1/users/me
Authorization: Bearer {access_token}
```

**Request Body:**
```json
{
  "first_name": "John",
  "last_name": "Doe",
  "about": "Software developer and tech enthusiast",
  "province": "California",
  "district": "San Francisco",
  "latitude": 37.7749,
  "longitude": -122.4194
}
```

### Follow/Unfollow User

```http
POST /api/v1/users/{user_id}/follow
DELETE /api/v1/users/{user_id}/follow
Authorization: Bearer {access_token}
```

---

## Posts

### Get Feed

```http
GET /api/v1/posts?sort_by=recent&limit=20&offset=0
```

**Query Parameters:**
- `sort_by`: `recent`, `trending`, `nearby` (default: `recent`)
- `type`: `FEED`, `EVENT`, `SELL`, `PULL`
- `province`: Filter by province
- `category_id`: Filter by category (for SELL posts)
- `latitude`, `longitude`, `radius_km`: Location-based filtering
- `limit`: Results per page (default: 20, max: 100)
- `offset`: Pagination offset

**Response:** `200 OK`
```json
{
  "success": true,
  "message": "Feed retrieved successfully",
  "data": [
    {
      "id": "uuid",
      "type": "FEED",
      "title": "My awesome post",
      "description": "This is a great post!",
      "author": {
        "user_id": "uuid",
        "first_name": "John",
        "last_name": "Doe",
        "full_name": "John Doe",
        "avatar": {...}
      },
      "attachments": [
        {
          "url": "https://cdn.example.com/image.webp",
          "width": 1200,
          "height": 800
        }
      ],
      "total_likes": 45,
      "total_comments": 12,
      "total_shares": 3,
      "liked_by_me": false,
      "bookmarked_by_me": false,
      "created_at": "2025-10-16T10:00:00Z"
    }
  ]
}
```

### Create Post

```http
POST /api/v1/posts
Authorization: Bearer {access_token}
```

**Request Body (FEED Post):**
```json
{
  "type": "FEED",
  "description": "This is my post description",
  "attachments": ["url1", "url2"],
  "visibility": "PUBLIC",
  "latitude": 37.7749,
  "longitude": -122.4194,
  "province": "California"
}
```

**Request Body (SELL Post):**
```json
{
  "type": "SELL",
  "title": "iPhone 15 Pro",
  "description": "Brand new, sealed",
  "price": 999.99,
  "currency": "USD",
  "category_id": "uuid",
  "attachments": ["url1", "url2"],
  "latitude": 37.7749,
  "longitude": -122.4194,
  "contact_no": "+1234567890"
}
```

**Request Body (EVENT Post):**
```json
{
  "type": "EVENT",
  "title": "Tech Meetup 2025",
  "description": "Join us for networking",
  "start_date": "2025-11-01T18:00:00Z",
  "start_time": "2025-11-01T18:00:00Z",
  "end_date": "2025-11-01T22:00:00Z",
  "latitude": 37.7749,
  "longitude": -122.4194
}
```

### Like/Unlike Post

```http
POST /api/v1/posts/{post_id}/like
DELETE /api/v1/posts/{post_id}/like
Authorization: Bearer {access_token}
```

### Bookmark/Unbookmark Post

```http
POST /api/v1/posts/{post_id}/bookmark
DELETE /api/v1/posts/{post_id}/bookmark
Authorization: Bearer {access_token}
```

---

## Comments

### Get Post Comments

```http
GET /api/v1/posts/{post_id}/comments?limit=20&offset=0
```

**Response:** `200 OK`
```json
{
  "success": true,
  "message": "Comments retrieved successfully",
  "data": [
    {
      "id": "uuid",
      "text": "Great post!",
      "author": {
        "user_id": "uuid",
        "full_name": "Jane Doe",
        "avatar": {...}
      },
      "total_likes": 5,
      "total_replies": 2,
      "created_at": "2025-10-16T10:05:00Z"
    }
  ]
}
```

### Create Comment

```http
POST /api/v1/posts/{post_id}/comments
Authorization: Bearer {access_token}
```

**Request Body:**
```json
{
  "text": "This is my comment",
  "parent_comment_id": "uuid" // Optional, for replies
}
```

---

## Business Profiles

### Get My Businesses

```http
GET /api/v1/businesses
Authorization: Bearer {access_token}
```

### Create Business

```http
POST /api/v1/businesses
Authorization: Bearer {access_token}
```

**Request Body:**
```json
{
  "name": "Tech Solutions Inc",
  "description": "We provide tech solutions",
  "address": "123 Main St, San Francisco, CA",
  "phone_number": "+1234567890",
  "email": "contact@techsolutions.com",
  "website": "https://techsolutions.com",
  "latitude": 37.7749,
  "longitude": -122.4194,
  "category_ids": ["uuid1", "uuid2"]
}
```

### Set Business Hours

```http
POST /api/v1/businesses/{business_id}/hours
Authorization: Bearer {access_token}
```

**Request Body:**
```json
{
  "hours": [
    {
      "day": "Monday",
      "open_time": "09:00",
      "close_time": "17:00",
      "is_closed": false
    },
    {
      "day": "Sunday",
      "is_closed": true
    }
  ]
}
```

---

## Search & Discovery

### Global Search

```http
GET /api/v1/search?query=tech&type=all&limit=20
```

**Query Parameters:**
- `query`: Search query (min 2 characters)
- `type`: `all`, `posts`, `users`, `businesses` (default: `all`)
- `limit`: Results per page
- `offset`: Pagination offset
- `latitude`, `longitude`, `radius_km`: Optional location filtering

**Response:** `200 OK`
```json
{
  "success": true,
  "message": "Search completed successfully",
  "data": {
    "posts": [...],
    "users": [...],
    "businesses": [...],
    "total": 45
  }
}
```

### Map-based Discovery

```http
GET /api/v1/discover?latitude=37.7749&longitude=-122.4194&radius_km=5
```

**Query Parameters:**
- `latitude`: Required
- `longitude`: Required
- `radius_km`: Radius in kilometers (max 100)
- `type`: Optional post type filter
- `limit`: Max results (default: 100, max: 500)

**Response:** `200 OK`
```json
{
  "success": true,
  "message": "Discovery completed successfully",
  "data": {
    "posts": [
      {
        "id": "uuid",
        "type": "SELL",
        "title": "iPhone 15",
        "location": {
          "latitude": 37.7750,
          "longitude": -122.4195
        },
        "distance": 0.5,
        "price": 999.99
      }
    ],
    "businesses": [
      {
        "id": "uuid",
        "name": "Tech Store",
        "location": {...},
        "distance": 1.2
      }
    ],
    "total": 15
  }
}
```

---

## Chat & Messaging

### WebSocket Connection

```http
GET /api/v1/chat/ws
Authorization: Bearer {access_token}
Upgrade: websocket
```

**WebSocket Message Format:**
```json
{
  "type": "message",
  "payload": {
    "conversation_id": "uuid",
    "content": "Hello!",
    "message_type": "TEXT"
  }
}
```

### Get Conversations

```http
GET /api/v1/chat/conversations?limit=20&offset=0
Authorization: Bearer {access_token}
```

### Send Message

```http
POST /api/v1/chat/messages
Authorization: Bearer {access_token}
```

**Request Body:**
```json
{
  "recipient_id": "uuid",
  "content": "Hello!",
  "message_type": "TEXT"
}
```

### Get Messages

```http
GET /api/v1/chat/conversations/{conversation_id}/messages?limit=50&offset=0
Authorization: Bearer {access_token}
```

---

## Notifications

### Get Notifications

```http
GET /api/v1/notifications?unread_only=false&limit=20
Authorization: Bearer {access_token}
```

### Register FCM Token

```http
POST /api/v1/notifications/fcm-token
Authorization: Bearer {access_token}
```

**Request Body:**
```json
{
  "token": "fcm_device_token_here"
}
```

### Update Notification Settings

```http
PUT /api/v1/notifications/settings
Authorization: Bearer {access_token}
```

**Request Body:**
```json
{
  "category": "MESSAGES",
  "push_pref": true
}
```

---

## Error Handling

All errors follow this format:

```json
{
  "success": false,
  "message": "Error description",
  "error": "Detailed error message"
}
```

### HTTP Status Codes

- `200` - Success
- `201` - Created
- `400` - Bad Request (validation error)
- `401` - Unauthorized (invalid/missing token)
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found
- `409` - Conflict (duplicate resource)
- `429` - Too Many Requests (rate limited)
- `500` - Internal Server Error

### Common Error Responses

**Validation Error (400):**
```json
{
  "success": false,
  "message": "Validation failed",
  "error": "email: must be a valid email address"
}
```

**Unauthorized (401):**
```json
{
  "success": false,
  "message": "Unauthorized",
  "error": "Invalid or expired token"
}
```

**Rate Limited (429):**
```json
{
  "success": false,
  "message": "Too many requests",
  "error": "Please try again in 60 seconds"
}
```

---

## Rate Limits

- **Authentication endpoints**: 5 requests per 15 minutes
- **General API**: 1000 requests per hour
- **File uploads**: 10 uploads per minute
- **Post creation**: 10 posts per hour

---

## Pagination

All list endpoints support pagination:

```http
GET /api/v1/posts?limit=20&offset=0
```

**Response includes metadata:**
```json
{
  "success": true,
  "data": [...],
  "meta": {
    "limit": 20,
    "offset": 0,
    "total": 150
  }
}
```

---

## Best Practices

1. **Always use HTTPS** in production
2. **Include Authorization header** for protected endpoints
3. **Handle rate limits** with exponential backoff
4. **Validate input** on client side before sending
5. **Use pagination** for list endpoints
6. **Cache static content** (avatars, images)
7. **Implement retry logic** for failed requests
8. **Handle WebSocket reconnections** gracefully

---

## Support

For API support, please contact:
- **Email**: dev@hamsaya.com
- **GitHub Issues**: https://github.com/hamsaya/backend/issues
