# Production Deployment Guide

**Status**: ✅ **Ready for Production**

This guide covers deploying the Hamsaya Backend API to production using Docker Compose.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [SSL/TLS Setup](#ssltls-setup)
- [Deployment](#deployment)
- [Monitoring](#monitoring)
- [Backups](#backups)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Server Requirements

- **OS**: Ubuntu 20.04 LTS or later (recommended)
- **CPU**: Minimum 4 cores (8+ recommended)
- **RAM**: Minimum 8GB (16GB+ recommended)
- **Storage**: Minimum 100GB SSD
- **Network**: Public IP address with ports 80 and 443 accessible

### Software Requirements

```bash
# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Verify installation
docker --version
docker-compose --version
```

---

## Architecture

The production deployment includes:

1. **Nginx** - Reverse proxy, SSL termination, rate limiting
2. **API (2 replicas)** - Go backend application
3. **PostgreSQL + PostGIS** - Primary database
4. **Redis** - Caching and session storage
5. **MinIO** - S3-compatible object storage

### Network Architecture

```
Internet
   ↓
Nginx (80/443)
   ↓
API (8080) ← → PostgreSQL (5432)
               ↓
               Redis (6379)
               ↓
               MinIO (9000)
```

All services except Nginx run on a private Docker network.

---

## Quick Start

### 1. Clone Repository

```bash
git clone https://github.com/your-org/hamsaya-backend.git
cd hamsaya-backend
```

### 2. Create Production Environment File

```bash
cp .env.example .env.prod
nano .env.prod  # Edit with your values
```

### 3. Generate SSL Certificates

See [SSL/TLS Setup](#ssltls-setup) section below.

### 4. Deploy

```bash
docker-compose -f docker-compose.prod.yml up -d
```

---

## Configuration

### Environment Variables

Create `.env.prod` with the following variables:

```bash
# Database
DB_NAME=hamsaya
DB_USER=postgres
DB_PASSWORD=<strong-password>  # REQUIRED

# Redis
REDIS_PASSWORD=<strong-password>  # REQUIRED

# JWT
JWT_SECRET=<64-char-random-string>  # REQUIRED

# Storage (MinIO)
STORAGE_ACCESS_KEY=<minio-access-key>  # REQUIRED
STORAGE_SECRET_KEY=<minio-secret-key>  # REQUIRED
STORAGE_BUCKET_NAME=hamsaya
CDN_URL=https://cdn.hamsaya.app  # Optional

# OAuth (Optional)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
FACEBOOK_APP_ID=
FACEBOOK_APP_SECRET=
APPLE_CLIENT_ID=
APPLE_TEAM_ID=
APPLE_KEY_ID=
APPLE_PRIVATE_KEY=

# Firebase Cloud Messaging (Optional)
FIREBASE_PROJECT_ID=
FIREBASE_CREDENTIALS_PATH=/app/firebase-credentials.json

# Email (SMTP)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=noreply@hamsaya.app
SMTP_PASSWORD=<app-password>
EMAIL_FROM=noreply@hamsaya.app

# CORS
CORS_ALLOWED_ORIGINS=https://hamsaya.app,https://www.hamsaya.app,https://admin.hamsaya.app

# Geocoding (Optional)
GEOCODING_API_KEY=<google-maps-api-key>
GEOCODING_PROVIDER=google

# Monitoring (Optional)
SENTRY_DSN=<sentry-dsn>
PROMETHEUS_ENABLED=false

# Nginx
MINIO_BROWSER_URL=https://minio.hamsaya.app
```

### Generate Secure Secrets

```bash
# Generate JWT secret (64 characters)
openssl rand -hex 32

# Generate strong passwords
openssl rand -base64 32
```

---

## SSL/TLS Setup

### Option 1: Let's Encrypt (Recommended)

#### 1. Install Certbot

```bash
sudo apt-get update
sudo apt-get install certbot
```

#### 2. Obtain Certificate

```bash
# Stop Nginx if running
docker-compose -f docker-compose.prod.yml stop nginx

# Get certificate
sudo certbot certonly --standalone \
  -d hamsaya.app \
  -d www.hamsaya.app \
  --email admin@hamsaya.app \
  --agree-tos \
  --no-eff-email

# Copy certificates
sudo cp /etc/letsencrypt/live/hamsaya.app/fullchain.pem nginx/ssl/
sudo cp /etc/letsencrypt/live/hamsaya.app/privkey.pem nginx/ssl/
sudo chmod 644 nginx/ssl/*

# Restart Nginx
docker-compose -f docker-compose.prod.yml up -d nginx
```

#### 3. Auto-Renewal

```bash
# Add cron job for renewal
sudo crontab -e

# Add this line:
0 3 * * * certbot renew --quiet --post-hook "cp /etc/letsencrypt/live/hamsaya.app/*.pem /path/to/project/nginx/ssl/ && docker-compose -f /path/to/project/docker-compose.prod.yml restart nginx"
```

### Option 2: Self-Signed (Development/Testing Only)

```bash
cd nginx/ssl
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout privkey.pem \
  -out fullchain.pem \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=hamsaya.app"
```

---

## Deployment

### Initial Deployment

```bash
# 1. Build images
docker-compose -f docker-compose.prod.yml build

# 2. Start services
docker-compose -f docker-compose.prod.yml up -d

# 3. Check status
docker-compose -f docker-compose.prod.yml ps

# 4. View logs
docker-compose -f docker-compose.prod.yml logs -f api

# 5. Run database migrations
docker-compose -f docker-compose.prod.yml exec api /app/migrate up
```

### Health Checks

```bash
# Check all services
curl http://localhost/health

# Check database
docker-compose -f docker-compose.prod.yml exec postgres psql -U postgres -d hamsaya -c "SELECT version();"

# Check Redis
docker-compose -f docker-compose.prod.yml exec redis redis-cli -a $REDIS_PASSWORD ping

# Check MinIO
curl http://localhost:9000/minio/health/live
```

### Updates

```bash
# 1. Pull latest code
git pull origin main

# 2. Rebuild images
docker-compose -f docker-compose.prod.yml build --no-cache

# 3. Rolling update (zero downtime)
docker-compose -f docker-compose.prod.yml up -d --no-deps --build api

# 4. Run migrations
docker-compose -f docker-compose.prod.yml exec api /app/migrate up

# 5. Verify
docker-compose -f docker-compose.prod.yml ps
```

---

## Monitoring

### Docker Stats

```bash
docker stats
```

### Logs

```bash
# All services
docker-compose -f docker-compose.prod.yml logs -f

# Specific service
docker-compose -f docker-compose.prod.yml logs -f api

# Last 100 lines
docker-compose -f docker-compose.prod.yml logs --tail=100 api

# Nginx access logs
docker-compose -f docker-compose.prod.yml exec nginx tail -f /var/log/nginx/access.log
```

### Health Endpoints

- **API Health**: `https://hamsaya.app/health`
- **API Ready**: `https://hamsaya.app/health/ready`
- **API Live**: `https://hamsaya.app/health/live`
- **Database Stats**: `https://hamsaya.app/health/db-stats`

---

## Backups

### Database Backup

```bash
# Create backup
docker-compose -f docker-compose.prod.yml exec postgres pg_dump -U postgres -d hamsaya -F c -f /tmp/backup.dump

# Copy backup to host
docker cp hamsaya-postgres-prod:/tmp/backup.dump ./backups/backup-$(date +%Y%m%d-%H%M%S).dump

# Restore backup
docker cp ./backups/backup-20241016-120000.dump hamsaya-postgres-prod:/tmp/restore.dump
docker-compose -f docker-compose.prod.yml exec postgres pg_restore -U postgres -d hamsaya -F c /tmp/restore.dump
```

### Automated Backups

Create `backup.sh`:

```bash
#!/bin/bash
BACKUP_DIR="/backups"
DATE=$(date +%Y%m%d-%H%M%S)

# Database backup
docker-compose -f docker-compose.prod.yml exec -T postgres pg_dump -U postgres -d hamsaya -F c > "$BACKUP_DIR/db-$DATE.dump"

# Redis backup (RDB file)
docker cp hamsaya-redis-prod:/data/dump.rdb "$BACKUP_DIR/redis-$DATE.rdb"

# MinIO data (optional - can be large)
# docker cp hamsaya-minio-prod:/data "$BACKUP_DIR/minio-$DATE"

# Clean old backups (keep last 7 days)
find "$BACKUP_DIR" -name "*.dump" -mtime +7 -delete
find "$BACKUP_DIR" -name "*.rdb" -mtime +7 -delete

# Upload to S3/backup service (optional)
# aws s3 cp "$BACKUP_DIR/db-$DATE.dump" s3://hamsaya-backups/
```

Add to crontab:

```bash
0 2 * * * /path/to/backup.sh >> /var/log/hamsaya-backup.log 2>&1
```

---

## Troubleshooting

### Service Won't Start

```bash
# Check service status
docker-compose -f docker-compose.prod.yml ps

# View logs
docker-compose -f docker-compose.prod.yml logs <service-name>

# Check resource usage
docker stats

# Restart specific service
docker-compose -f docker-compose.prod.yml restart <service-name>
```

### Database Connection Issues

```bash
# Check if PostgreSQL is running
docker-compose -f docker-compose.prod.yml exec postgres pg_isready

# Check connections
docker-compose -f docker-compose.prod.yml exec postgres psql -U postgres -d hamsaya -c "SELECT count(*) FROM pg_stat_activity;"

# View active queries
docker-compose -f docker-compose.prod.yml exec postgres psql -U postgres -d hamsaya -c "SELECT pid, query, state FROM pg_stat_activity WHERE state != 'idle';"
```

### High Memory Usage

```bash
# Check which service is using memory
docker stats --no-stream

# Restart service to clear memory
docker-compose -f docker-compose.prod.yml restart <service-name>

# Adjust resource limits in docker-compose.prod.yml
```

### SSL Certificate Issues

```bash
# Check certificate expiry
openssl x509 -in nginx/ssl/fullchain.pem -noout -dates

# Renew Let's Encrypt certificate
sudo certbot renew --force-renewal

# Copy new certificate
sudo cp /etc/letsencrypt/live/hamsaya.app/*.pem nginx/ssl/

# Restart Nginx
docker-compose -f docker-compose.prod.yml restart nginx
```

---

## Security Best Practices

1. **Firewall Configuration**:
   ```bash
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   sudo ufw enable
   ```

2. **Regular Updates**:
   ```bash
   # Update Docker images
   docker-compose -f docker-compose.prod.yml pull
   docker-compose -f docker-compose.prod.yml up -d
   ```

3. **Secrets Management**:
   - Use environment variables, never commit secrets
   - Consider using Docker Secrets or HashiCorp Vault for sensitive data

4. **Monitoring**:
   - Set up monitoring (Prometheus, Grafana, or cloud provider)
   - Configure alerts for high CPU, memory, disk usage
   - Monitor error rates and response times

5. **Backups**:
   - Automated daily backups
   - Test restore procedures regularly
   - Store backups in multiple locations

---

## Performance Tuning

### PostgreSQL

Already optimized in `docker-compose.prod.yml`:
- `max_connections=200`
- `shared_buffers=256MB`
- `effective_cache_size=1GB`

### Nginx

- Rate limiting configured
- Gzip compression enabled
- Connection pooling to backend

### API

- 2 replicas for load balancing
- Health checks enabled
- Connection pooling to database and Redis

---

## Support

For issues or questions:

- **Documentation**: Check this guide and `CLAUDE.md`
- **Logs**: Always check logs first: `docker-compose logs -f`
- **GitHub Issues**: Report bugs and request features

---

**Last Updated**: October 16, 2025
