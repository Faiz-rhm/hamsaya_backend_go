# Health Check Endpoints

This document describes all health check endpoints available in the Hamsaya Backend API for monitoring, observability, and debugging.

---

## Overview

The application provides multiple health check endpoints designed for different purposes:

- **Liveness probes**: Check if the application is alive and running
- **Readiness probes**: Check if the application can serve traffic
- **Startup probes**: Check if the application has started successfully
- **Metrics**: System and runtime metrics for monitoring
- **Statistics**: Database and Redis connection statistics
- **Version info**: Build and version information

All health endpoints are available at the `/health/*` path (no versioning) and do not require authentication.

---

## Endpoints

### 1. Basic Health Check

```http
GET /health
```

**Purpose**: Simple health check to verify the API is running.

**Response**: `200 OK`
```json
{
  "success": true,
  "message": "OK",
  "data": {
    "status": "healthy",
    "timestamp": "2025-10-16T10:00:00Z"
  }
}
```

**Use Case**: Quick check to see if the service is up.

---

### 2. Liveness Probe

```http
GET /health/live
```

**Purpose**: Kubernetes liveness probe. Checks if the application is alive.

**Response**: `200 OK`
```json
{
  "success": true,
  "message": "OK",
  "data": {
    "status": "alive"
  }
}
```

**Use Case**:
- Kubernetes liveness probe
- Determine if the container should be restarted
- This endpoint always returns 200 if the process is running

**Kubernetes Configuration**:
```yaml
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 3
```

---

### 3. Readiness Probe

```http
GET /health/ready
```

**Purpose**: Kubernetes readiness probe. Checks if the application is ready to serve traffic.

**Healthy Response**: `200 OK`
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

**Degraded Response**: `503 Service Unavailable`
```json
{
  "success": false,
  "message": "Service degraded",
  "data": {
    "status": "degraded",
    "timestamp": "2025-10-16T10:00:00Z",
    "services": {
      "database": "healthy",
      "redis": "unhealthy: connection refused"
    }
  }
}
```

**Checks Performed**:
- Database connection (2-second timeout)
- Redis connection (2-second timeout)

**Use Case**:
- Kubernetes readiness probe
- Load balancer health checks
- Determine if traffic should be routed to this instance
- Shows partial health status (degraded) when some services fail

**Kubernetes Configuration**:
```yaml
readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

---

### 4. Startup Probe

```http
GET /health/startup
```

**Purpose**: Kubernetes startup probe. Checks if the application has started successfully.

**Success Response**: `200 OK`
```json
{
  "success": true,
  "message": "Application started",
  "data": {
    "status": "started",
    "started_at": "2025-10-16T09:00:00Z",
    "uptime": "1h30m45s"
  }
}
```

**Failure Response**: `503 Service Unavailable`
```json
{
  "success": false,
  "message": "Database not ready",
  "error": "connection timeout"
}
```

**Checks Performed**:
- Database connection (5-second timeout)
- Redis connection (5-second timeout)

**Use Case**:
- Kubernetes startup probe for slow-starting applications
- Initial health check after deployment
- Gives the application more time to initialize before marking as unhealthy

**Kubernetes Configuration**:
```yaml
startupProbe:
  httpGet:
    path: /health/startup
    port: 8080
  initialDelaySeconds: 0
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 30  # Allow up to 5 minutes for startup
```

---

### 5. Database Statistics

```http
GET /health/db-stats
```

**Purpose**: Get database connection pool statistics.

**Response**: `200 OK`
```json
{
  "success": true,
  "message": "Database statistics",
  "data": {
    "acquired_conns": 5,
    "canceled_acquire_count": 0,
    "constructing_conns": 0,
    "empty_acquire_count": 123,
    "idle_conns": 20,
    "max_conns": 25,
    "total_conns": 25,
    "new_conns_count": 25,
    "max_lifetime_destroy_count": 0,
    "max_idle_destroy_count": 0
  }
}
```

**Metrics Explained**:
- `acquired_conns`: Currently active connections
- `idle_conns`: Idle connections available in the pool
- `max_conns`: Maximum allowed connections
- `total_conns`: Total connections in the pool
- `new_conns_count`: Total connections created since startup
- `canceled_acquire_count`: Number of canceled connection acquisitions
- `empty_acquire_count`: Connections acquired without waiting
- `max_lifetime_destroy_count`: Connections destroyed due to max lifetime
- `max_idle_destroy_count`: Connections destroyed due to max idle time

**Use Case**:
- Monitor database connection pool utilization
- Detect connection leaks
- Tune connection pool settings
- Debug performance issues

**Alert Thresholds**:
- `acquired_conns` near `max_conns`: Pool exhaustion (increase max_conns or find connection leaks)
- High `canceled_acquire_count`: Timeout issues (increase pool size or optimize queries)
- `idle_conns` consistently at 0: Insufficient pool size

---

### 6. Redis Statistics

```http
GET /health/redis-stats
```

**Purpose**: Get Redis server information and statistics.

**Response**: `200 OK`
```json
{
  "success": true,
  "message": "Redis statistics",
  "data": {
    "connected": true,
    "db_size": 1234,
    "info": "# Server\nredis_version:7.0.0\n...",
    "memory_info": "# Memory\nused_memory:1048576\n..."
  }
}
```

**Metrics Included**:
- `connected`: Redis connection status
- `db_size`: Number of keys in the database
- `info`: Full Redis INFO output (server, clients, memory, stats, etc.)
- `memory_info`: Redis memory usage details

**Use Case**:
- Monitor Redis memory usage
- Check Redis server version
- Monitor connected clients
- Debug Redis performance issues

---

### 7. Version Information

```http
GET /health/version
```

**Purpose**: Get application version and build information.

**Response**: `200 OK`
```json
{
  "success": true,
  "message": "Version information",
  "data": {
    "version": "1.0.0",
    "build_time": "2025-10-16T08:00:00Z",
    "git_commit": "abc123def456",
    "go_version": "go1.21.0"
  }
}
```

**Build Information**:
- `version`: Application version (set via `-ldflags` during build)
- `build_time`: When the binary was built
- `git_commit`: Git commit hash
- `go_version`: Go compiler version used

**Setting Build Info**:
```bash
# During build
go build -ldflags="-X 'main.version=1.0.0' \
                    -X 'main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)' \
                    -X 'main.gitCommit=$(git rev-parse HEAD)'" \
         -o bin/server cmd/server/main.go
```

**Use Case**:
- Verify deployed version
- Track deployments
- Debug version-specific issues
- Audit trail

---

### 8. System Metrics

```http
GET /health/metrics
```

**Purpose**: Get application runtime and system metrics.

**Response**: `200 OK`
```json
{
  "success": true,
  "message": "System metrics",
  "data": {
    "uptime_seconds": 5445.123,
    "uptime_human": "1h30m45.123s",
    "goroutines": 42,
    "memory": {
      "alloc_mb": 15.2,
      "total_alloc_mb": 250.5,
      "sys_mb": 25.3,
      "num_gc": 23,
      "gc_pause_ns": 123456,
      "heap_alloc_mb": 15.2,
      "heap_sys_mb": 20.0,
      "heap_idle_mb": 4.8,
      "heap_in_use_mb": 15.2,
      "heap_released_mb": 0.0,
      "heap_objects": 125432
    },
    "cpu": {
      "num_cpu": 8,
      "goos": "linux",
      "goarch": "amd64"
    }
  }
}
```

**Metrics Explained**:

**Uptime**:
- `uptime_seconds`: Seconds since application started
- `uptime_human`: Human-readable uptime

**Goroutines**:
- `goroutines`: Number of active goroutines (should be relatively stable)

**Memory**:
- `alloc_mb`: Currently allocated heap memory (MB)
- `total_alloc_mb`: Cumulative allocated memory (MB)
- `sys_mb`: Total memory obtained from OS (MB)
- `num_gc`: Number of completed GC cycles
- `gc_pause_ns`: Last GC pause duration (nanoseconds)
- `heap_alloc_mb`: Heap memory in use (MB)
- `heap_sys_mb`: Heap memory obtained from OS (MB)
- `heap_idle_mb`: Idle heap memory (MB)
- `heap_in_use_mb`: Heap memory in use (MB)
- `heap_released_mb`: Heap memory released to OS (MB)
- `heap_objects`: Number of allocated heap objects

**CPU**:
- `num_cpu`: Number of logical CPUs
- `goos`: Operating system
- `goarch`: Architecture

**Use Case**:
- Monitor memory usage and detect memory leaks
- Track goroutine leaks
- Monitor GC performance
- System capacity planning
- Performance optimization

**Alert Thresholds**:
- `alloc_mb` continuously increasing: Potential memory leak
- `goroutines` continuously increasing: Goroutine leak
- High `num_gc`: Frequent garbage collection (may need optimization)
- `gc_pause_ns` > 10ms: Long GC pauses affecting performance

---

## Monitoring Setup

### Prometheus Integration

The `/health/metrics` endpoint can be scraped by Prometheus for monitoring:

**Prometheus Config**:
```yaml
scrape_configs:
  - job_name: 'hamsaya-backend'
    scrape_interval: 30s
    metrics_path: '/health/metrics'
    static_configs:
      - targets: ['api:8080']
```

**Grafana Dashboard Metrics**:
- Uptime: `uptime_seconds`
- Memory Usage: `alloc_mb`, `heap_in_use_mb`
- Goroutines: `goroutines`
- GC Performance: `num_gc`, `gc_pause_ns`

### Health Check Monitoring

**Recommended Checks**:
```bash
# Every 30 seconds
curl -f http://localhost:8080/health/ready || alert

# Every 5 minutes
curl http://localhost:8080/health/db-stats | jq '.data.acquired_conns'
curl http://localhost:8080/health/metrics | jq '.data.memory.alloc_mb'
```

### Docker Health Check

**docker-compose.yml**:
```yaml
services:
  api:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health/ready"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s
```

**Dockerfile**:
```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health/ready || exit 1
```

---

## Troubleshooting

### Service Not Ready

**Symptom**: `/health/ready` returns 503

**Diagnosis**:
```bash
# Check which service is unhealthy
curl http://localhost:8080/health/ready | jq '.data.services'

# Check database
curl http://localhost:8080/health/db-stats

# Check Redis
curl http://localhost:8080/health/redis-stats
```

**Common Causes**:
- Database connection pool exhausted
- Redis connection timeout
- Network connectivity issues
- Service startup in progress

### Memory Issues

**Symptom**: High memory usage or OOM kills

**Diagnosis**:
```bash
# Check current memory
curl http://localhost:8080/health/metrics | jq '.data.memory'

# Monitor over time
watch -n 5 'curl -s http://localhost:8080/health/metrics | jq ".data.memory.alloc_mb"'
```

**Common Causes**:
- Memory leak in application code
- Too many concurrent goroutines
- Large response payloads
- Insufficient container memory limits

### Connection Pool Exhaustion

**Symptom**: Slow queries or timeouts

**Diagnosis**:
```bash
# Check pool usage
curl http://localhost:8080/health/db-stats | jq '.data | {acquired: .acquired_conns, max: .max_conns, idle: .idle_conns}'
```

**Solution**:
- Increase `DB_MAX_CONNS` in environment
- Fix connection leaks (ensure proper cleanup)
- Optimize query performance

---

## Best Practices

1. **Use appropriate probes in Kubernetes**:
   - Startup probe for initial health check
   - Liveness probe to detect deadlocks
   - Readiness probe to control traffic routing

2. **Monitor trends, not just current values**:
   - Memory usage over time
   - Goroutine count over time
   - Connection pool usage

3. **Set up alerts**:
   - Service unhealthy for > 2 minutes
   - Memory usage > 80% of limit
   - Connection pool > 90% utilized
   - Goroutine count continuously increasing

4. **Regular checks**:
   - Verify version after deployments
   - Check database stats during high load
   - Monitor GC performance

5. **Use structured logging**:
   - All health checks are logged
   - Include request IDs for tracing
   - Monitor logs for health check failures

---

## Security Considerations

- Health endpoints do not require authentication
- Health endpoints do not expose sensitive data
- Internal system details are minimal in responses
- Use firewall rules to restrict health endpoint access if needed
- Version endpoint can be disabled in production if needed

---

## Examples

### Check All Health Endpoints

```bash
#!/bin/bash

echo "=== Basic Health ==="
curl -s http://localhost:8080/health | jq .

echo -e "\n=== Liveness ==="
curl -s http://localhost:8080/health/live | jq .

echo -e "\n=== Readiness ==="
curl -s http://localhost:8080/health/ready | jq .

echo -e "\n=== Startup ==="
curl -s http://localhost:8080/health/startup | jq .

echo -e "\n=== Version ==="
curl -s http://localhost:8080/health/version | jq .

echo -e "\n=== Metrics ==="
curl -s http://localhost:8080/health/metrics | jq '.data | {uptime: .uptime_human, goroutines, memory: .memory | {alloc_mb, heap_in_use_mb, num_gc}}'

echo -e "\n=== DB Stats ==="
curl -s http://localhost:8080/health/db-stats | jq '.data | {acquired_conns, idle_conns, max_conns}'

echo -e "\n=== Redis Stats ==="
curl -s http://localhost:8080/health/redis-stats | jq '.data | {connected, db_size}'
```

### Monitor Memory Over Time

```bash
#!/bin/bash

while true; do
  timestamp=$(date '+%Y-%m-%d %H:%M:%S')
  memory=$(curl -s http://localhost:8080/health/metrics | jq -r '.data.memory.alloc_mb')
  goroutines=$(curl -s http://localhost:8080/health/metrics | jq -r '.data.goroutines')
  echo "$timestamp - Memory: ${memory}MB, Goroutines: $goroutines"
  sleep 10
done
```

### Check Before Deployment

```bash
#!/bin/bash

# Verify service is ready
if curl -f -s http://localhost:8080/health/ready > /dev/null; then
  echo "✓ Service is ready"
else
  echo "✗ Service is not ready"
  exit 1
fi

# Verify version
version=$(curl -s http://localhost:8080/health/version | jq -r '.data.version')
echo "Version: $version"

# Check database connections
db_conns=$(curl -s http://localhost:8080/health/db-stats | jq '.data.acquired_conns')
echo "Active DB connections: $db_conns"
```

---

## API Reference Summary

| Endpoint | Purpose | Auth Required | Timeout |
|----------|---------|---------------|---------|
| `/health` | Basic health check | No | N/A |
| `/health/live` | Liveness probe | No | N/A |
| `/health/ready` | Readiness probe | No | 2s |
| `/health/startup` | Startup probe | No | 5s |
| `/health/db-stats` | Database statistics | No | N/A |
| `/health/redis-stats` | Redis statistics | No | 2s |
| `/health/version` | Version information | No | N/A |
| `/health/metrics` | System metrics | No | N/A |
