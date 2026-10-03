# Health Checks

Authara exposes separate liveness and readiness endpoints plus a health-check
command for container runtimes and orchestration systems.

This allows systems such as:

- Docker
- Kubernetes
- load balancers
- container platforms

to automatically detect unhealthy instances.

---

# Container Health Check

The official Authara container image includes a built-in health check.

Example:

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD ["/app/authara", "healthcheck"]
```

The container runtime periodically executes this command.

If the command exits successfully, the container is considered healthy.

If the command fails, the container is considered unhealthy.

---

# Health Check Command

Authara provides a dedicated command for health checks:

```
authara healthcheck
```

The command requests `http://127.0.0.1:8080/auth/ready`. It succeeds only when
the running Authara process is accepting traffic, PostgreSQL responds with the
required schema version, and configured Redis is available. The HTTP request is
bounded by a two-second client timeout; dependency checks share a one-second
server-side timeout.

It exits with:

| Exit Code | Meaning |
|---|---|
| `0` | Service is healthy |
| non-zero | Service is unhealthy |

The command does not produce user-facing output and is intended for automated checks.

During graceful shutdown Authara marks this endpoint unavailable before closing
the HTTP listener. Health checks then return a non-zero result while existing
HTTP requests and background deliveries drain.

---

# HTTP Endpoints

| Endpoint | Purpose | Dependencies checked |
|---|---|---|
| `/auth/live` | Confirms the HTTP process is alive | None |
| `/auth/ready` | Confirms the instance can receive application traffic | PostgreSQL, schema, and configured Redis |
| `/auth/health` | Compatibility alias for `/auth/ready` | Same as `/auth/ready` |

Readiness requires lifecycle readiness, a successful PostgreSQL ping, the exact
schema version required by the running Core binary, and a successful Redis ping
when `AUTHARA_CACHE_PROVIDER=redis`. Dependency checks share a one-second
timeout. An outage removes the instance from readiness without terminating it,
allowing dependency clients to recover when service returns.

When Prometheus metrics are enabled, each dependency check also updates its
result counter, duration histogram, and last observed status. The effective
replica state is exported as `authara_readiness_status`. See
[Prometheus Metrics](metrics.md) for the complete metric semantics.

Use `/auth/live` for Kubernetes liveness probes and `/auth/ready` for readiness
probes. Do not use the database-dependent endpoint as a liveness probe: a shared
database outage should not restart every Authara replica.

Example Kubernetes container configuration:

```yaml
spec:
  terminationGracePeriodSeconds: 15
  containers:
    - name: authara
      image: ghcr.io/authara-org/authara-core:v0.21.1
      ports:
        - name: http
          containerPort: 8080
      startupProbe:
        httpGet:
          path: /auth/live
          port: http
        periodSeconds: 2
        failureThreshold: 30
      livenessProbe:
        httpGet:
          path: /auth/live
          port: http
        periodSeconds: 10
        failureThreshold: 3
      readinessProbe:
        httpGet:
          path: /auth/ready
          port: http
        periodSeconds: 5
        failureThreshold: 2
```

The startup and liveness probes verify only the running HTTP process. Kubernetes
uses readiness to add or remove the pod from service routing. Keep the
termination grace period above Authara's ten-second drain deadline.

---

# Typical Usage

Container runtimes execute the health check automatically.

Example Docker behavior:

1. container starts
2. Docker waits for `start-period`
3. Docker executes the health check command periodically
4. if checks fail repeatedly, the container is marked unhealthy

Container orchestration systems may then:

- restart the container
- remove it from load balancing
- trigger alerts

---

# Purpose

Health checks allow operators to ensure that:

- the Authara process is running
- ready instances can reach PostgreSQL
- the service can be restarted automatically if necessary

They are an important part of production deployments.

---

# Summary

Authara provides a built-in health check command designed for container environments.

The official container image configures this command automatically through Docker's `HEALTHCHECK` directive.
