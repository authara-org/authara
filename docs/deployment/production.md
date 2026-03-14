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

Some features, such as the current in-memory rate limiter, are instance-local.

This means rate limiting is **not shared across instances** unless a shared limiter backend is added in the future.

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
