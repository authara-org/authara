# Production

In production, Authara is typically deployed as part of a larger application stack.

Authara runs as a dedicated authentication service behind a reverse proxy or gateway and is usually mounted under:

```
/auth
```

Example routing:

```text
/auth/* → Authara
/*      → Application
```

This allows authentication to live on the same origin as the application while remaining operationally separate.

---

# Typical production topology

A common production deployment looks like this:

```text
Client
  ↓
Load Balancer / Reverse Proxy / Authara Gateway
  ├── /auth/* → Authara Core
  └── /*      → Application
```

Authara Core connects to PostgreSQL internally.

---

# Same-origin deployment

Authara is deployed on the **same origin** as the application.

Example:

```text
https://example.com/auth/* → Authara
https://example.com/*      → Application
```

This simplifies:

- cookie handling
- CSRF protection
- browser-based authentication flows
- SSR and HTMX integrations

In this model, `PUBLIC_URL` should be set to the application origin **without** `/auth`.

Example:

```env
PUBLIC_URL=https://example.com
```

---

# HTTP caching

Authara Core marks dynamic responses with `Cache-Control: no-store`. Reverse
proxies, gateways, CDNs and load balancers must preserve this header and must
not force-cache `/auth/*` responses.

The exception is `/auth/static/*`: production asset filenames contain a content
fingerprint and Authara serves them with
`Cache-Control: public, max-age=31536000, immutable`. Proxies may retain that
long-lived policy for those assets.

---

# HTTPS

Production deployments should always use **HTTPS**.

This is required for secure cookie handling and to protect authentication traffic in transit.

TLS is usually terminated by:

- a load balancer
- a reverse proxy
- Authara Gateway (depending on configuration)
- ingress infrastructure

---

# Database

Authara requires PostgreSQL.

Production PostgreSQL should be treated as a durable infrastructure dependency.

Important considerations include:

- backups
- restore procedures
- upgrade planning
- monitoring
- secure credential management

Authara does not manage PostgreSQL for you.

---

# Migrations

Schema changes must be applied explicitly before starting a new Authara version that requires them.

Authara does **not** auto-migrate.

Production upgrade flow typically looks like this:

1. deploy the new migrations image
2. apply migrations
3. deploy the new Authara Core version
4. verify startup and health

See:

- [Migrations](../operations/migrations.md)

---

# Configuration

Production configuration is provided through environment variables.

This usually includes:

- PostgreSQL connection settings
- `PUBLIC_URL`
- JWT issuer and signing keys
- session settings
- OAuth provider configuration
- rate limiting settings

Secrets should be stored in a secure secret management system rather than committed files.

Challenge-policy and rate-limit values may be changed by an operator only when
the corresponding environment variable is absent. Supplying one explicitly
locks that individual value to deployment configuration. Secrets and bootstrap
settings are never runtime-editable. See
[Operator runtime settings](../operations/runtime-settings.md).

See:

- [Configuration Reference](../configuration/reference.md)

---

# Scaling

Authara Core is designed to run as a separate service.

In multi-instance deployments, operators should consider:

- shared PostgreSQL access
- consistent environment configuration
- load balancing behavior
- schema compatibility during rollout

Some features, such as the default in-memory rate limiter, are instance-local.

Use the same access-token revocation mode on every replica. The recommended
production profile is `AUTHARA_CACHE_PROVIDER=redis` with
`AUTHARA_ACCESS_TOKEN_REVOCATION_MODE=immediate`; it shares rate limits and
rejects revoked access tokens across instances. A deliberately minimal
deployment may use `AUTHARA_CACHE_PROVIDER=noop` with the explicitly required
`AUTHARA_ACCESS_TOKEN_REVOCATION_MODE=expiry`, which permits issued access
tokens to remain usable for at most 10 minutes after logout or an authorization
change. Do not mix these profiles during a rolling deployment.

Runtime-setting writes take effect immediately on the accepting Core replica.
Other replicas reconcile the PostgreSQL revision every two seconds. Plan for
that bounded delay during concurrent rollouts and policy changes.

One replica at a time holds the PostgreSQL-backed `cleanup` lease and runs all
retention cleanup schedules. Followers continue serving traffic and retry
leadership without failing health checks. Graceful shutdown releases leadership
immediately; after a crash or partition another replica takes over after the
30-second lease expires. Cleanup passes are fenced by the lease generation,
limited to one deterministic delete category per database batch, and stop after
a bounded time. A pass that reaches its row or time budget yields to other due
jobs and resumes after a five-second cooldown.

Cleanup intervals are startup-only environment settings. Configure the same
values on every replica: `AUTHARA_SESSION_CLEANUP_INTERVAL`,
`AUTHARA_EMAIL_CLEANUP_INTERVAL`, `AUTHARA_WEBHOOK_CLEANUP_INTERVAL`,
`AUTHARA_ADMIN_AUDIT_CLEANUP_INTERVAL`, and
`AUTHARA_SECURITY_EVENT_CLEANUP_INTERVAL`. A newly elected leader runs each
cleanup once immediately and then follows those intervals. During a rolling
upgrade, singleton behavior is guaranteed only after replicas running the old
unleased workers have drained.

Operator audit cleanup uses a fixed 24-hour schedule. Configure its startup-only
retention with `AUTHARA_OPERATOR_AUDIT_RETENTION_DAYS`, which defaults to `180`.

## Process shutdown and restart behavior

Authara treats `SIGTERM` and `SIGINT` as normal termination requests. It first
marks `/auth/ready` and its compatibility alias `/auth/health` unavailable and
stops accepting new HTTP and background work, then drains HTTP requests, email
and webhook deliveries, runtime-setting reconciliation, and maintenance work
under one shared 10-second deadline. `/auth/live` remains independent of
external dependencies and should be used only as a liveness probe; readiness
includes bounded PostgreSQL, required-schema, and configured-Redis checks.

A clean signal-driven shutdown exits with status `0`. Listener failures,
unexpected server or worker termination, shutdown timeouts, and resource-close
failures exit non-zero. Normal `http.ErrServerClosed` completion is not an
error. A second termination signal during the drain uses the operating system's
default behavior and can force the process to exit.

Configure the deployment platform's termination grace period above 10 seconds
so Authara can consume its full drain budget and still leave time for the
container runtime to observe the exit. In-flight durable email and webhook jobs
that cannot settle before the deadline remain protected by their processing
leases and are recovered by the stale-job reapers.

---

# Operational model

Authara is designed to behave like infrastructure.

This means:

- startup should be deterministic
- schema changes should be explicit
- runtime should not mutate shared state unexpectedly
- failures should be visible and fail fast

Authara validates schema compatibility during startup and refuses to start if the database schema is incompatible with the running binary.

---

# Summary

A production Authara deployment usually consists of:

- Authara Core
- PostgreSQL
- a reverse proxy or gateway
- your application

Authara is normally mounted under `/auth` on the same origin as the application, with explicit migrations and operator-controlled upgrades.
