# Structured logs

Authara Core writes one JSON object per line to standard output. The schema is
designed for container log collectors and keeps operational events searchable
without turning logs into a second store for request or customer data.

Every record contains these resource fields:

| Field | Meaning |
| --- | --- |
| `timestamp` | UTC RFC 3339 timestamp |
| `level` | `debug`, `info`, `warn`, or `error` |
| `service` | Always `authara` |
| `environment` | `AUTHARA_LOG_ENVIRONMENT`, or `APP_ENV` when unset |
| `version` | Running Core version |
| `instance_id` | Random UUID for the running process |
| `event` | Stable event category |

Event-specific fields are typed and bounded. Stable categories include
`request_completed`, `authentication_outcome`, `webhook_delivery`,
`queue_outcome`, `worker_outcome`, `maintenance_outcome`, and
`configuration_status`. Fields such as `component`, `operation`, `outcome`,
`error_code`, `route`, and `status_class` use closed values suitable for log
queries and dashboards.

## Privacy boundary

Core does not log raw errors, request paths or query strings, request or
response bodies, headers, IP addresses, user agents, email addresses, user or
organization identifiers, provider identifiers, SQL, secrets, or configuration
values. HTTP records use the registered route template, such as
`/auth/users/{userID}`, rather than the requested path. Unmatched or unsafe
routes are recorded as `unmatched`.

`X-Request-ID` is accepted only when it is a canonical UUID v4 or v7. Missing
or malformed values are replaced and the resulting identifier is returned in
the response header. This permits correlation without accepting arbitrary
caller-controlled log content.

Unknown events or fields fail closed as `log_record_dropped`. Each output line
is valid JSON and is capped at 16 KiB, including its newline.

## Configuration

Set `LOG_LEVEL` to control the minimum emitted level. Set
`AUTHARA_LOG_ENVIRONMENT` to a bounded deployment name (1–32 letters, numbers,
dots, underscores, or hyphens). If it is unset, Core uses `APP_ENV`.

Log collectors should parse the JSON body and promote only low-cardinality
resource fields required by their deployment. Core itself does not prescribe a
collector, backend, or label policy.
