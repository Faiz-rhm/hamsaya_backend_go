# Deployment Guide - Hamsaya Backend

This guide covers deploying the Hamsaya backend to production environments.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Setup](#environment-setup)
3. [Database Setup](#database-setup)
4. [Docker Deployment](#docker-deployment)
5. [Kubernetes Deployment](#kubernetes-deployment)
6. [Monitoring & Logging](#monitoring--logging)
7. [Security Checklist](#security-checklist)
8. [Backup & Recovery](#backup--recovery)

---

## Prerequisites

### Required Services

- **PostgreSQL 15+** with PostGIS extension
- **Redis 7+**
- **MinIO** or AWS S3 for object storage
- **Firebase** project for push notifications (optional)
- **SSL Certificate** for HTTPS

### System Requirements

- **CPU**: 2+ cores recommended
- **RAM**: 4GB+ recommended
- **Disk**: 20GB+ for application and logs
- **Network**: Public IP or load balancer

---

## Environment Setup

### 1. Clone Repository

```bash
git clone https://github.com/hamsaya/backend.git
cd backend
```

### 2. Configure Environment Variables

Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` with production values:

```bash
# Server Configuration
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
ENV=production
LOG_LEVEL=info

# Database (PostgreSQL with PostGIS)
DB_HOST=postgres.example.com
DB_PORT=5432
DB_NAME=hamsaya_prod
DB_USER=hamsaya_user
DB_PASSWORD=STRONG_PASSWORD_HERE
DB_SSL_MODE=require
DB_MAX_CONNS=100
DB_MIN_CONNS=10
DB_MAX_CONN_LIFETIME=1h
DB_MAX_CONN_IDLE_TIME=30m

# Redis
REDIS_HOST=redis.example.com
REDIS_PORT=6379
REDIS_PASSWORD=REDIS_PASSWORD_HERE
REDIS_DB=0

# JWT Configuration
JWT_SECRET=VERY_STRONG_SECRET_KEY_AT_LEAST_32_CHARS
JWT_ACCESS_TOKEN_DURATION=15m
JWT_REFRESH_TOKEN_DURATION=168h

# OAuth Providers
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret

APPLE_CLIENT_ID=your_apple_client_id
APPLE_TEAM_ID=your_apple_team_id
APPLE_KEY_ID=your_apple_key_id
APPLE_PRIVATE_KEY=your_apple_private_key

FACEBOOK_APP_ID=your_facebook_app_id
FACEBOOK_APP_SECRET=your_facebook_app_secret

# Object Storage (MinIO/S3)
STORAGE_ENDPOINT=s3.amazonaws.com
STORAGE_ACCESS_KEY=your_access_key
STORAGE_SECRET_KEY=your_secret_key
STORAGE_BUCKET_NAME=hamsaya-media
STORAGE_USE_SSL=true
STORAGE_REGION=us-east-1
CDN_URL=https://cdn.hamsaya.com

# Firebase (for push notifications)
FIREBASE_PROJECT_ID=hamsaya-prod
FIREBASE_CREDENTIALS_PATH=/etc/secrets/firebase-credentials.json

# Geocoding (Google Maps API)
GEOCODING_API_KEY=your_google_maps_api_key
GEOCODING_PROVIDER=google

# Rate Limiting
RATE_LIMIT_REQUESTS_PER_HOUR=1000
RATE_LIMIT_AUTH_ATTEMPTS=5
RATE_LIMIT_AUTH_WINDOW=15m

# Email (SMTP)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=noreply@hamsaya.com
SMTP_PASSWORD=your_smtp_password
EMAIL_FROM=Hamsaya <noreply@hamsaya.com>

# CORS
CORS_ALLOWED_ORIGINS=https://app.hamsaya.com,https://admin.hamsaya.com
CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS
CORS_ALLOWED_HEADERS=Content-Type,Authorization
CORS_ALLOW_CREDENTIALS=true

# Monitoring
SENTRY_DSN=https://your-sentry-dsn
PROMETHEUS_ENABLED=true
```

### 3. Security Hardening

**Generate Strong Secrets:**
```bash
# Generate JWT secret
openssl rand -base64 32

# Generate random password
openssl rand -base64 24
```

**Set Proper File Permissions:**
```bash
chmod 600 .env
chmod 600 /etc/secrets/firebase-credentials.json
```

---

## Database Setup

### 1. Create Database

```sql
-- Connect as postgres user
CREATE DATABASE hamsaya_prod;
CREATE USER hamsaya_user WITH PASSWORD 'STRONG_PASSWORD';
GRANT ALL PRIVILEGES ON DATABASE hamsaya_prod TO hamsaya_user;

-- Connect to hamsaya_prod database
\c hamsaya_prod

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "postgis";

-- Grant permissions
GRANT ALL ON SCHEMA public TO hamsaya_user;
```

### 2. Run Migrations

```bash
# Build migration tool
go build -o bin/migrate cmd/migrate/main.go

# Run migrations
./bin/migrate up
```

### 3. Verify Setup

```bash
# Check migrations
./bin/migrate status

# Test database connection
psql "postgresql://hamsaya_user:PASSWORD@postgres.example.com:5432/hamsaya_prod?sslmode=require" -c "SELECT version();"
```

---

## Docker Deployment

### 1. Build Production Image

```bash
docker build -t hamsaya-backend:latest -f Dockerfile.prod .
```

**Dockerfile.prod:**
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s" \
    -o /app/bin/server cmd/server/main.go

# Production stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/bin/server .
COPY --from=builder /app/migrations ./migrations

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser && \
    chown -R appuser:appgroup /root

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health/live || exit 1

CMD ["./server"]
```

### 2. Run with Docker Compose

**docker-compose.prod.yml:**
```yaml
version: '3.8'

services:
  api:
    image: hamsaya-backend:latest
    container_name: hamsaya-api
    restart: unless-stopped
    ports:
      - "8080:8080"
    env_file:
      - .env
    volumes:
      - ./logs:/var/log/hamsaya
      - ./secrets:/etc/secrets:ro
    depends_on:
      - postgres
      - redis
    networks:
      - hamsaya-network
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8080/health/live"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  postgres:
    image: postgis/postgis:15-3.3
    container_name: hamsaya-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: hamsaya_prod
      POSTGRES_USER: hamsaya_user
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./backups:/backups
    networks:
      - hamsaya-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U hamsaya_user"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: hamsaya-redis
    restart: unless-stopped
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    networks:
      - hamsaya-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3

  nginx:
    image: nginx:alpine
    container_name: hamsaya-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
      - ./logs/nginx:/var/log/nginx
    depends_on:
      - api
    networks:
      - hamsaya-network

volumes:
  postgres_data:
  redis_data:

networks:
  hamsaya-network:
    driver: bridge
```

### 3. Start Services

```bash
docker-compose -f docker-compose.prod.yml up -d
```

### 4. View Logs

```bash
# All services
docker-compose -f docker-compose.prod.yml logs -f

# Specific service
docker-compose -f docker-compose.prod.yml logs -f api
```

---

## Kubernetes Deployment

### 1. Create Namespace

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: hamsaya-prod
```

### 2. Create Secrets

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: hamsaya-secrets
  namespace: hamsaya-prod
type: Opaque
stringData:
  DB_PASSWORD: "your_db_password"
  REDIS_PASSWORD: "your_redis_password"
  JWT_SECRET: "your_jwt_secret"
  STORAGE_SECRET_KEY: "your_storage_secret"
```

### 3. Create ConfigMap

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: hamsaya-config
  namespace: hamsaya-prod
data:
  SERVER_PORT: "8080"
  ENV: "production"
  LOG_LEVEL: "info"
  DB_HOST: "postgres.hamsaya-prod.svc.cluster.local"
  DB_PORT: "5432"
  DB_NAME: "hamsaya_prod"
  REDIS_HOST: "redis.hamsaya-prod.svc.cluster.local"
  REDIS_PORT: "6379"
```

### 4. Deploy Application

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hamsaya-api
  namespace: hamsaya-prod
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hamsaya-api
  template:
    metadata:
      labels:
        app: hamsaya-api
    spec:
      containers:
      - name: api
        image: hamsaya-backend:latest
        ports:
        - containerPort: 8080
        envFrom:
        - configMapRef:
            name: hamsaya-config
        - secretRef:
            name: hamsaya-secrets
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### 5. Create Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: hamsaya-api
  namespace: hamsaya-prod
spec:
  selector:
    app: hamsaya-api
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

### 6. Apply Configurations

```bash
kubectl apply -f namespace.yaml
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

---

## Monitoring & Logging

### 1. Prometheus Metrics

The API exposes Prometheus metrics at `/metrics`:

```yaml
# prometheus-config.yaml
scrape_configs:
  - job_name: 'hamsaya-api'
    static_configs:
      - targets: ['hamsaya-api:8080']
```

**Key Metrics:**
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request latency
- `db_connections_active` - Active database connections
- `cache_hits_total` - Cache hit rate

### 2. Structured Logging

Logs are output in JSON format:

```json
{
  "level": "info",
  "ts": "2025-10-16T10:00:00Z",
  "caller": "handlers/post_handler.go:45",
  "msg": "Post created",
  "user_id": "uuid",
  "post_id": "uuid",
  "request_id": "req-123"
}
```

### 3. Log Aggregation

**Using ELK Stack:**
```bash
# Configure Filebeat
filebeat.inputs:
- type: container
  paths:
    - '/var/lib/docker/containers/*/*.log'

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
```

**Using Loki:**
```yaml
# promtail-config.yaml
clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: containers
    static_configs:
      - targets:
          - localhost
        labels:
          job: containerlogs
          __path__: /var/lib/docker/containers/*/*log
```

### 4. Health Checks

The application provides comprehensive health check endpoints for monitoring:

```bash
# Basic health check
curl http://localhost:8080/health

# Liveness probe (is the app running?)
curl http://localhost:8080/health/live

# Readiness probe (can the app serve traffic?)
curl http://localhost:8080/health/ready

# Startup probe (has the app started successfully?)
curl http://localhost:8080/health/startup

# Database connection pool statistics
curl http://localhost:8080/health/db-stats

# Redis server statistics
curl http://localhost:8080/health/redis-stats

# Application version and build info
curl http://localhost:8080/health/version

# System metrics (memory, CPU, goroutines, uptime)
curl http://localhost:8080/health/metrics
```

**Health Check Response Format:**

```json
{
  "success": true,
  "message": "Service ready",
  "data": {
    "status": "ready",
    "timestamp": "2025-10-16T10:00:00Z",
    "services": {
      "database": "healthy",
      "redis": "healthy"
    }
  }
}
```

**Degraded State:**
When services are partially unhealthy, the `/health/ready` endpoint returns HTTP 503 with:
```json
{
  "success": false,
  "message": "Service degraded",
  "data": {
    "status": "degraded",
    "timestamp": "2025-10-16T10:00:00Z",
    "services": {
      "database": "healthy",
      "redis": "unhealthy: connection timeout"
    }
  }
}
```

---

## Security Checklist

- [ ] **HTTPS Only**: Enforce HTTPS in production
- [ ] **Strong Secrets**: Use 32+ character secrets
- [ ] **Environment Variables**: Never commit secrets to Git
- [ ] **Rate Limiting**: Configure appropriate rate limits
- [ ] **CORS**: Restrict allowed origins
- [ ] **Database**: Use SSL/TLS connections
- [ ] **Redis**: Enable password authentication
- [ ] **File Permissions**: Restrict access to sensitive files
- [ ] **Container Security**: Run as non-root user
- [ ] **API Keys**: Rotate regularly
- [ ] **Monitoring**: Set up alerts for anomalies
- [ ] **Backups**: Automated daily backups
- [ ] **Firewall**: Restrict network access

---

## Backup & Recovery

### 1. Database Backup

**Automated Backup Script:**
```bash
#!/bin/bash
# backup.sh

DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backups"
DB_NAME="hamsaya_prod"

# Create backup
pg_dump -Fc \
  -h postgres.example.com \
  -U hamsaya_user \
  -d $DB_NAME \
  -f $BACKUP_DIR/backup_${DATE}.dump

# Compress
gzip $BACKUP_DIR/backup_${DATE}.dump

# Delete backups older than 30 days
find $BACKUP_DIR -name "backup_*.dump.gz" -mtime +30 -delete
```

**Cron Job:**
```bash
# Run daily at 2 AM
0 2 * * * /path/to/backup.sh
```

### 2. Database Restore

```bash
# Restore from backup
pg_restore -Fc \
  -h postgres.example.com \
  -U hamsaya_user \
  -d hamsaya_prod \
  -c backup_20251016_020000.dump.gz
```

### 3. Redis Backup

```bash
# Manual save
redis-cli -a $REDIS_PASSWORD SAVE

# Copy RDB file
cp /var/lib/redis/dump.rdb /backups/redis_backup_$(date +%Y%m%d).rdb
```

---

## Troubleshooting

### Common Issues

**1. Cannot connect to database**
```bash
# Check database is running
docker ps | grep postgres

# Test connection
psql "postgresql://user:pass@host:5432/db" -c "SELECT 1"

# Check logs
docker logs hamsaya-postgres
```

**2. High memory usage**
```bash
# Check container stats
docker stats hamsaya-api

# Review connection pool settings
# Adjust DB_MAX_CONNS in .env
```

**3. Slow queries**
```bash
# Enable slow query log in PostgreSQL
ALTER DATABASE hamsaya_prod SET log_min_duration_statement = 1000;

# Check slow queries
SELECT * FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;
```

---

## Scaling

### Horizontal Scaling

```bash
# Docker Swarm
docker service scale hamsaya-api=5

# Kubernetes
kubectl scale deployment hamsaya-api --replicas=5 -n hamsaya-prod
```

### Load Balancing

**Nginx Configuration:**
```nginx
upstream hamsaya_backend {
    least_conn;
    server api1:8080;
    server api2:8080;
    server api3:8080;
}

server {
    listen 443 ssl http2;
    server_name api.hamsaya.com;

    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;

    location / {
        proxy_pass http://hamsaya_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

## Support

For deployment support:
- **Documentation**: https://docs.hamsaya.com
- **GitHub Issues**: https://github.com/hamsaya/backend/issues
- **Email**: devops@hamsaya.com
