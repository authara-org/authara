# Migrations

Authara requires a PostgreSQL database schema.

Schema changes are managed through **explicit migrations**.

Authara does **not** automatically mutate the database schema at runtime.

Instead:

1. migrations are applied explicitly
2. Authara starts against the resulting schema
3. Authara validates the schema version at startup

This behavior is intentional and keeps database changes predictable and operator-controlled.

---

# Why migrations are separate

Database schema changes are part of the **operational lifecycle** of Authara.

They are not runtime behavior.

This means:

- schema changes are applied explicitly
- startup is deterministic
- upgrades are controlled
- runtime does not mutate shared state unexpectedly

Authara treats schema management as an operator responsibility.

---

# Migration Image

Authara provides a dedicated migration image.

This image contains:

- the SQL migration files
- the migration runner
- the logic required to apply schema changes

It does **not** start the Authara server.

It runs migrations and exits.

---

# Applying migrations

Migrations are typically applied using the migration image before starting Authara.

Example:

```bash
docker run --rm \
  --env-file .env \
  ghcr.io/authara-org/authara-migrations:latest
```

This applies all pending migrations and exits.

---

# Configuration

The migration image uses the same PostgreSQL connection variables as Authara Core.

Required variables include:

```env
POSTGRESQL_HOST=postgres
POSTGRESQL_PORT=5432
POSTGRESQL_DATABASE=authara
POSTGRESQL_USERNAME=authara
POSTGRESQL_PASSWORD=authara
```

These variables may be provided through:

- `.env`
- container environment variables
- CI secrets
- orchestration platforms

---

# Typical usage

Migrations should be applied:

- before the first startup
- before starting a newer Authara version with a changed schema
- in CI when testing schema compatibility

Typical operational flow:

1. Start PostgreSQL
2. Run migrations
3. Start Authara Core
4. Start the gateway and application

---

# Development

Migrations are required in development as well.

Even in local development, Authara expects the database schema to already exist.

This keeps development behavior consistent with staging and production.

Example local flow:

1. Start PostgreSQL
2. Run migrations
3. Start Authara

---

# Schema compatibility check

Authara validates the schema version at startup.

If the database schema does not match the version required by the running Authara binary, startup fails.

This prevents:

- partially upgraded deployments
- accidental runtime mismatches
- undefined database behavior

In other words:

> If Authara starts successfully, the schema version is compatible.

---

# Rollbacks

Rollback behavior depends on the migrations that have been applied.

Schema rollbacks should be treated as an advanced operational task.

Before rolling back:

- understand the migration contents
- evaluate possible data loss
- test the rollback path in a safe environment

Authara does not assume that rollbacks are always safe.

---

# Summary

Authara migrations are:

- explicit
- operator-controlled
- required in development and production
- validated through schema version checks at startup

This keeps schema evolution predictable and prevents hidden runtime database changes.
