# Viewing Logs, Metrics, and Traces

The backend emits **logs** (stdout + optional OTLP), **metrics** (Prometheus + optional OTLP), and **traces** (OTLP when configured). Here’s how to view them.

---

## 1. Logs (always)

**Where:** The process **stdout** (terminal or container logs).

When you run the server with `go run cmd/server/main.go` or `make run`, every request and application log goes to the terminal.

```bash
# Run server and watch logs in the same terminal
go run cmd/server/main.go
```

Example output:

```
INFO  Starting Hamsaya Backend API...
INFO  Configuration loaded  {"env": "development", "port": "8080"}
INFO  HTTP request  {"method": "GET", "path": "/health", "status": 200, "latency_ms": 1, "trace_id": "...", "span_id": "..."}
```

**Docker:** View API container logs:

```bash
docker compose logs -f api
```

---

## 2. Metrics (Prometheus)

**Where:** HTTP endpoint **`/metrics`** on the API (Prometheus text format).

- **URL:** `http://localhost:8080/metrics`
- **When:** Always (as long as the server is up).

Open in a browser or scrape with Prometheus:

```bash
curl http://localhost:8080/metrics
```

You’ll see:

- `http_requests_total` – request count by method, path, status
- `http_request_duration_seconds` – latency histogram
- `http_server_active_requests` – in-flight requests
- Plus DB, business, and (if present) Go runtime metrics

**Optional:** Run Prometheus and Grafana to graph these (see “All-in-one UI” below).

---

## 3. Traces + OTLP (when enabled)

Traces (and optionally metrics + logs) are sent via **OTLP** only when you set **`OTLP_ENDPOINT`** and **`OBSERVABILITY_ENABLED=true`**.

**Enable in `.env`:**

```env
OBSERVABILITY_ENABLED=true
OTLP_ENDPOINT=localhost:4317
```

Then run an **OTLP receiver** (e.g. OpenTelemetry Collector or an all-in-one stack). Your backend will send traces (and, if configured, metrics and logs) to that endpoint.

---

## 4. View traces in Jaeger (easiest local UI)

To see **traces** (request flows, spans) in a UI:

1. **Start Jaeger** (receives OTLP from your backend):

   ```bash
   cd hamsaya_backend_go
   docker compose -f docker-compose.observability.yml up -d
   ```

2. **Enable OTLP in `.env`:**

   ```env
   OBSERVABILITY_ENABLED=true
   OTLP_ENDPOINT=localhost:4317
   ```

3. **Start your API** (e.g. `go run cmd/server/main.go`).

4. **Open Jaeger:** http://localhost:16686  
   - Service: **hamsaya-backend**  
   - Click **Find Traces**.

## 5. All-in-one UI (Grafana + Loki + Tempo + Mimir)

For **logs + traces + metrics** in one place, use Grafana’s stack (e.g. [grafana/otel-lgtm](https://github.com/grafana/otel-lgtm) or Grafana Cloud). Point your backend’s `OTLP_ENDPOINT` at the collector.

### Grafana Cloud or other backends

- **Grafana Cloud:** Create a stack, get OTLP endpoint and (if needed) API key, set `OTLP_ENDPOINT` (and any auth env vars your SDK supports).
- **Datadog / Honeycomb / etc.:** Use their OTLP endpoint and docs; set `OTLP_ENDPOINT` (and any required auth) in `.env`.

---

## Quick reference

| What        | How to view |
|------------|-------------|
| **Logs**   | Terminal stdout, or `docker compose logs -f api`; or Grafana Explore (Loki) when OTLP is enabled. |
| **Metrics**| Browser: `http://localhost:8080/metrics`; or Grafana dashboards when you scrape Prometheus / use Mimir. |
| **Traces** | Only when `OTLP_ENDPOINT` is set: Jaeger, Grafana (Tempo), or another OTLP backend. |

---

## Env vars (recap)

| Variable                 | Purpose |
|-------------------------|--------|
| `OBSERVABILITY_ENABLED` | `true` to enable OTel (traces, OTLP metrics/logs when endpoint is set). |
| `OTLP_ENDPOINT`         | Host:port for OTLP gRPC (e.g. `localhost:4317`). If set, traces + optional metrics/logs are sent here. |
| `TRACE_SAMPLING_RATE`   | 0–1 (e.g. `1.0` = 100%, `0.1` = 10%). |
| `LOG_LEVEL`             | Zap level: `debug`, `info`, `warn`, `error`. |
