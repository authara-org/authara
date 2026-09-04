# Prometheus Metrics

Prometheus metric collection is disabled by default. Enable it at startup with:

```dotenv
AUTHARA_METRICS_ENABLED=true
```

When enabled, Authara Core exposes metrics on:

```text
GET /metrics
```

The endpoint uses the Prometheus text exposition format and supports OpenMetrics
content negotiation. It does not require an Authara user session.

When disabled, Authara does not create the metrics registry or collectors, install
the HTTP instrumentation middleware, record background-job metrics, or register
the `/metrics` route.

## Built-in metrics

Authara exports:

- `authara_build_info` with the running Authara version
- `authara_http_requests_total` by HTTP method, route pattern, and status code
- `authara_http_request_duration_seconds` by HTTP method, route pattern, and status code
- `authara_http_response_size_bytes` by HTTP method, route pattern, and status code
- `authara_http_requests_in_flight` for in-flight requests
- `authara_background_jobs_total` by worker and outcome (`succeeded`, `retried`, `failed`, or `error`)
- `authara_background_job_duration_seconds` by worker and outcome
- standard `go_sql_*` database pool metrics for the primary PostgreSQL connection
- standard `go_*` runtime metrics
- standard `process_*` CPU, memory, file descriptor, and process-start metrics where supported
- `promhttp_metric_handler_*` metrics describing Prometheus scrapes

HTTP metrics use route patterns such as `/auth/api/v1/organizations/{organizationID}`.
Raw request paths are never used as labels, which keeps metric cardinality bounded
and avoids exposing identifiers through metric labels. Unknown HTTP methods are
reported as `OTHER`, and unmatched routes are reported as `unmatched`.

Background metrics currently cover the `email` and `webhook` workers. They make
terminal failures, retries, and slow external delivery visible without including
recipient addresses, event IDs, or other high-cardinality labels.

Database pool metrics include open, in-use, and idle connections as well as
connection wait counts, wait duration, and connection churn. Useful signals include:

```promql
# Sustained pool utilization
go_sql_in_use_connections{db_name="primary"}
  / go_sql_max_open_connections{db_name="primary"}

# Requests forced to wait for a database connection
rate(go_sql_wait_count_total{db_name="primary"}[5m])

# Background jobs reaching a terminal failure or an unexpected state error
increase(authara_background_jobs_total{outcome=~"failed|error"}[10m])
```

## Prometheus configuration

Scrape the Core service directly on its configured HTTP address:

```yaml
scrape_configs:
  - job_name: authara-core
    static_configs:
      - targets: ["authara-core:8080"]
```

Authara is normally exposed to browsers through an `/auth/*` reverse-proxy route,
so `/metrics` can remain reachable only from the internal monitoring network.
If the Core port is exposed directly, restrict `/metrics` at the load balancer,
reverse proxy, firewall, or network-policy layer.

## Adding application metrics

When enabled, the observability service owns a private Prometheus registry rather
than using the process-global registry. Application modules can register
additional collectors through `App.Observability.Registerer()` and they will
appear on the same `/metrics` endpoint.
