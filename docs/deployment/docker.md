# Docker

Authara is typically run with Docker.

A minimal deployment usually consists of:

- PostgreSQL
- Authara Core
- Authara Gateway
- your application

Authara itself is provided as container images.

---

# Images

Authara uses separate images for separate concerns.

Typical images include:

- `ghcr.io/authara-org/authara-core` — main authentication server
- `ghcr.io/authara-org/authara-gateway` — reverse proxy / gateway
- `ghcr.io/authara-org/authara-migrations` — database migrations

This separation keeps runtime responsibilities explicit.

---

# Typical topology

A common Docker setup looks like this:

```text
Client
  ↓
Authara Gateway
  ├── /auth/* → Authara Core
  └── /*      → Application
```

Authara Core connects to PostgreSQL internally.

---

# Minimal Docker Compose example

```yaml
services:

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: authara
      POSTGRES_USER: authara
      POSTGRES_PASSWORD: authara

  app:
    image: nginx:alpine

  authara-migrations:
    image: ghcr.io/authara-org/authara-migrations:latest
    env_file:
      - .env
    depends_on:
      - postgres

  authara:
    image: ghcr.io/authara-org/authara-core:latest
    env_file:
      - .env
    depends_on:
      authara-migrations:
        condition: service_completed_successfully

  gateway:
    image: ghcr.io/authara-org/authara-gateway:latest
    ports:
      - "3000:3000"
    environment:
      GATEWAY_BIND: :3000
      AUTHARA_UPSTREAM: authara:8080
      APP_UPSTREAM: app:80
    depends_on:
      - authara
      - app
```

This example shows the network shape only.

Application-specific configuration is intentionally omitted.

---

# Environment Variables

Authara Core is configured through environment variables.

This is typically done using an `.env` file:

```yaml
env_file:
  - .env
```

The `.env` file usually contains:

- PostgreSQL connection details
- public URL
- JWT configuration
- session settings
- OAuth settings
- cache / Redis settings
- rate limiting settings

See:

- [Configuration Overview](../configuration/overview.md)
- [Configuration Reference](../configuration/reference.md)

---

# Startup order

A typical startup sequence is:

1. start PostgreSQL
2. run migrations
3. start Authara Core
4. start Authara Gateway
5. start the application

This ensures Authara starts against the expected schema.

See:

- [Migrations](../operations/migrations.md)

---

# Local development

For local development, Docker Compose is usually the simplest option.

A typical local stack uses:

- a local PostgreSQL container
- Authara Core
- Authara Gateway
- the application container or a local app process

Quick local setup is described in:

- [Quickstart](../quickstart.md)

---

# Summary

Docker is the recommended way to run Authara.

Typical deployments use separate images for:

- Authara Core
- Authara Gateway
- migrations

This keeps the system explicit, predictable, and easy to operate.
