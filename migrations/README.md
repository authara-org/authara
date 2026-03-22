# Authara Database Migrations

This directory contains the **database schema migrations required by Authara**.

Authara **does not manage database schema automatically**.  
Migrations must be applied **explicitly** before running the Authara server.

---

## Overview

- Authara requires a PostgreSQL database
- The database schema is versioned using SQL migrations
- Migrations are applied using `sql-migrate`
- Authara **will refuse to start** if the database schema version is incompatible

This design is intentional and ensures:
- explicit infrastructure changes
- safe upgrades and rollbacks
- no hidden side effects at runtime

---

## Migration Tooling

Authara provides a **dedicated migration runner Docker image** built from this directory.

The migration image:
- contains all SQL migrations
- contains the `sql-migrate` CLI
- does **not** start Authara
- exits after applying migrations

---

## Running Migrations (Docker – Recommended)

### Prerequisites

You must provide database connection details via environment variables, for example:

POSTGRESQL_HOST=localhost  
POSTGRESQL_PORT=5432  
POSTGRESQL_DATABASE=authara  
POSTGRESQL_USERNAME=authara  
POSTGRESQL_PASSWORD=secret  

### Apply migrations

docker run --rm \
  --env-file .env \
  ghcr.io/authara-org/authara-migrations:<version> \
  up -env=default -config=dbconfig.yaml

This command:
- connects to the database
- applies all pending migrations
- exits immediately

---

## Checking Migration Status

docker run --rm \
  --env-file .env \
  ghcr.io/authara-org/authara-migrations:<version> \
  status -env=default -config=dbconfig.yaml

---

## Rolling Back (Advanced)

docker run --rm \
  --env-file .env \
  ghcr.io/authara-org/authara-migrations:<version> \
  down -env=default -config=dbconfig.yaml

⚠️ **Warning:** Rolling back migrations may result in data loss.  
Only perform rollbacks if you fully understand the impact.

---

## Schema Compatibility Enforcement

Authara **validates schema compatibility on startup**.

If the database schema version does not match the version required by the Authara server, startup will fail with an error similar to:

database schema version mismatch  
current: 002_users  
required: 003_sessions  

In this case, apply the correct migrations before starting Authara.

---

## Design Principles

- Authara **does not own your database**
- Schema changes are **explicit**
- Migrations are **operator-controlled**
- Runtime behavior is **deterministic and safe**

This is infrastructure software — not an application that mutates shared state automatically.

---

## Summary

- Migrations are required to run Authara
- Migrations must be applied explicitly
- Authara will not auto-migrate
- Schema compatibility is enforced at startup

If Authara starts successfully, the database schema is guaranteed to be correct.
