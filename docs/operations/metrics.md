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
- `authara_email_queue_age_seconds` by bounded delivery outcome
- `authara_maintenance_leader` (`1` on the cleanup leader, `0` on followers)
- `authara_maintenance_lease_attempts_total` by acquisition/lifecycle outcome
- `authara_maintenance_runs_total` and `authara_maintenance_run_duration_seconds` by cleanup job and outcome
- `authara_maintenance_rows_processed_total` by cleanup job
- `authara_readiness_checks_total` by bounded dependency (`postgres`, `schema`, or `redis`) and result
- `authara_readiness_check_duration_seconds` by dependency and result
- `authara_readiness_dependency_status` for the last observed state of each checked dependency
- `authara_readiness_status` for the effective readiness state of the replica
- `authara_queue_jobs` by queue (`email` or `webhook`) and state (`pending`, `retry`, `processing`, or `failed`)
- `authara_queue_oldest_job_age_seconds` by queue and state
- `authara_queue_oldest_ready_age_seconds` for immediately claimable work
- `authara_queue_stuck_jobs` for processing leases older than the queue's configured stale threshold
- `authara_queue_snapshot_refreshes_total` and `authara_queue_snapshot_last_success_timestamp_seconds`
- `authara_background_polls_total`, `authara_background_poll_duration_seconds`, and `authara_background_poll_last_success_timestamp_seconds` by worker
- `authara_queue_reaper_runs_total`, `authara_queue_reaper_duration_seconds`, and `authara_queue_reaper_jobs_total`
- `authara_runtime_settings_reconciliations_total`, `authara_runtime_settings_reconciliation_duration_seconds`, `authara_runtime_settings_revision`, and `authara_runtime_settings_last_success_timestamp_seconds`
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

Queue gauges are refreshed every 15 seconds by a lifecycle-managed monitor. A
`pending` job has not yet been attempted; a `retry` job is pending after at least
one delivery attempt. `processing` means that a worker owns the job, and `failed`
is terminal. The oldest-ready gauge considers only pending jobs whose
`next_attempt_at` has passed, so scheduled retry backoff does not look like an
immediately blocked queue. A stuck job is still processing after the configured
email or webhook lease threshold.

If a snapshot query fails, Authara increments the corresponding failed refresh
counter and retains the previous queue gauges. Use the last-success timestamp to
distinguish a stable queue from stale telemetry. Email and webhook snapshots are
queried independently, so a failure for one is visible without suppressing the
other.

Poll metrics distinguish `claimed`, `empty`, and `failed`. Both `claimed` and
`empty` are successful database polls and advance the worker's last-success
timestamp. Reaper job outcomes distinguish stale jobs returned to `retry` from
jobs moved to terminal `failed` because their attempt or delivery deadline was
exhausted.

Runtime reconciliation reports `applied`, `unchanged`, or `failed`. The revision
gauge is the database revision currently applied by the replica; a successful
reconciliation advances the last-success timestamp even when there is no new
revision.

Readiness dependency status is updated by readiness probes. The overall status
is `0` during startup and graceful shutdown, or after a dependency check fails,
and `1` after a complete readiness check succeeds.

Maintenance outcomes distinguish completed, incomplete, failed, and canceled
cleanup passes. An incomplete pass reached its row or time budget and is queued
again after a short cooldown instead of waiting for the normal cleanup interval.
Lease outcomes include skipped acquisition, which is expected on healthy follower
replicas and must not be used as a readiness failure.

Maintenance job labels include `admin_audit` and `operator_audit` for their
respective audit-retention cleanup passes.

The rows-processed counter reports directly deleted root rows. Rows removed by
foreign-key cascades are intentionally not included.

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

# Cleanup failures by job
increase(authara_maintenance_runs_total{outcome="failed"}[30m])

# Dependency readiness failures
increase(authara_readiness_checks_total{result="failed"}[10m])

# Total queued work, including retries whose backoff has not elapsed
authara_queue_jobs{state=~"pending|retry"}

# A queue snapshot that has not refreshed recently
time() - authara_queue_snapshot_last_success_timestamp_seconds

# Claim/poller database failures
increase(authara_background_polls_total{result="failed"}[10m])

# Stale processing recovered or terminated by a reaper
increase(authara_queue_reaper_jobs_total[10m])

# Replicas that have not applied the same runtime-settings revision
max(authara_runtime_settings_revision) - min(authara_runtime_settings_revision)
```

Authara does not ship alert thresholds or paging rules. Appropriate limits
depend on an operator's traffic, worker concurrency, delivery retry policy, and
service-level objectives. The bounded metrics above are intended as inputs to
that deployment-specific policy.

## Related structured logs

Metrics identify the affected subsystem; structured logs provide the local
failure detail. Relevant messages include `email worker iteration failed`,
`webhook worker iteration failed`, `queue metrics snapshot failed`, stale-reaper
failures and recovery summaries, `cleanup lease acquisition failed`, `cleanup
pass failed`, and `runtime settings reconciliation failed`. Error records use an
`error` field, while queue, cleanup job, worker ID, retry/failed counts, and lease
generation are emitted as bounded fields where applicable. Job, event, and user
identifiers remain in logs rather than Prometheus labels.

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
