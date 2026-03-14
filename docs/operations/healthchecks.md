# Health Checks

Authara exposes a health check command that can be used by container runtimes and orchestration systems to verify that the service is running correctly.

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

The command performs a minimal internal check to verify that the server is operational.

It exits with:

| Exit Code | Meaning |
|---|---|
| `0` | Service is healthy |
| non-zero | Service is unhealthy |

The command does not produce user-facing output and is intended for automated checks.

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
- the container is functioning correctly
- the service can be restarted automatically if necessary

They are an important part of production deployments.

---

# Summary

Authara provides a built-in health check command designed for container environments.

The official container image configures this command automatically through Docker's `HEALTHCHECK` directive.
