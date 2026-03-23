# Database Connection Pooling

Authara uses a connection pool to manage PostgreSQL connections efficiently.

Correct configuration of the connection pool is **critical for performance, scalability, and system stability**.

---

## Overview

Each Authara instance maintains its own pool of database connections.

Instead of opening a new connection for every request, connections are reused from the pool.

This allows:

- lower latency
- reduced database overhead
- controlled concurrency

---

## How it works

At runtime:

1. Authara opens connections to PostgreSQL (up to `MAX_OPEN_CONNS`)
2. Idle connections are reused
3. New requests wait if no connection is available
4. Connections are recycled based on lifetime settings

---

## Configuration

### AUTHARA_DB_MAX_OPEN_CONNS

Maximum number of open connections.

This is the **most important setting**.

- Limits total concurrency toward the database
- Protects PostgreSQL from overload

Example:

```
AUTHARA_DB_MAX_OPEN_CONNS=40
```

---

### AUTHARA_DB_MAX_IDLE_CONNS

Maximum number of idle (pooled) connections.

- Keeps connections warm
- Reduces latency for bursts

Example:

```
AUTHARA_DB_MAX_IDLE_CONNS=20
```

---

### AUTHARA_DB_CONN_MAX_LIFETIME

Maximum lifetime of a connection.

- Forces periodic reconnection
- Prevents stale or broken connections

Example:

```
AUTHARA_DB_CONN_MAX_LIFETIME=30m
```

---

### AUTHARA_DB_CONN_MAX_IDLE_TIME

Maximum idle time before a connection is closed.

- Frees unused connections
- Helps reduce unnecessary DB load

Example:

```
AUTHARA_DB_CONN_MAX_IDLE_TIME=5m
```

---

## Sizing Guidelines

### Step 1 — Check PostgreSQL limit

```
SHOW max_connections;
```

Example:

```
max_connections = 200
```

---

### Step 2 — Divide across instances

Each Authara instance needs its own pool.

Formula:

```
max_connections / number_of_instances ≈ MAX_OPEN_CONNS
```

---

### Step 3 — Apply safety margin

Never use the full limit.

Recommended:

- use **20–50% of total capacity**
- leave room for:
  - migrations
  - admin tools
  - other services

---

### Example setups

#### Single instance

```
Postgres max_connections = 200

AUTHARA_DB_MAX_OPEN_CONNS = 40
AUTHARA_DB_MAX_IDLE_CONNS = 20
```

---

#### Two instances

```
Postgres max_connections = 200

Each instance:
AUTHARA_DB_MAX_OPEN_CONNS = 30–40
```

---

#### Four instances

```
Postgres max_connections = 200

Each instance:
AUTHARA_DB_MAX_OPEN_CONNS = 20–30
```

---

## Failure modes

### Too many connections

Error:

```
FATAL: sorry, too many clients already
```

Symptoms:

- login failures
- refresh failures
- HTTP 500 errors
- cascading auth failures

Cause:

- `MAX_OPEN_CONNS` too high
- too many instances
- missing pooling

---

### Too few connections

Symptoms:

- high latency
- slow login
- request queueing

Cause:

- pool too small for traffic

---

### Connection churn

Symptoms:

- unstable latency
- spikes under load

Cause:

- lifetime too low
- idle time too low

---

## Production recommendations

Good baseline:

```
AUTHARA_DB_MAX_OPEN_CONNS=40
AUTHARA_DB_MAX_IDLE_CONNS=20
AUTHARA_DB_CONN_MAX_LIFETIME=30m
AUTHARA_DB_CONN_MAX_IDLE_TIME=5m
```

---

## Scaling beyond 1k+ users

At higher scale, connection pooling alone is not enough.

Recommended:

### Use PgBouncer

PgBouncer sits between Authara and PostgreSQL and:

- multiplexes connections
- drastically reduces DB load
- enables much higher concurrency

Without PgBouncer:

- PostgreSQL becomes the bottleneck early
- connection limits are hit quickly

---

## Key Takeaway

Authara does not limit traffic — PostgreSQL does.

Connection pool settings are the **control layer** between:

- application concurrency
- database capacity

Correct tuning ensures:

- stable authentication flows
- predictable latency
- no cascading failures under load
