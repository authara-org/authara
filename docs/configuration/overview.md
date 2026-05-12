# Configuration Overview

Authara is configured entirely through **environment variables**.

Configuration is typically provided via:

- `.env` files
- container environments
- orchestration platforms (Kubernetes, ECS, etc.)

Authara does **not load configuration files directly**.

---

## Configuration categories

Authara configuration is organized into the following areas:

### Runtime

Controls environment and logging behavior.

Examples:

```
APP_ENV
LOG_LEVEL
```

---

### Database

Defines the PostgreSQL connection.

Examples:

```
POSTGRESQL_HOST
POSTGRESQL_DATABASE
POSTGRESQL_USERNAME
POSTGRESQL_PASSWORD
```

---

### Cache

Configures the optional cache backend.

→ See: `configuration/cache.md`

Examples:

```
AUTHARA_CACHE_PROVIDER
AUTHARA_REDIS_HOST
```

---

### Public URL

Defines the externally accessible base URL.

Used for:

- redirects
- OAuth callbacks

Example:

```
PUBLIC_URL
```

---

### Token lifetimes

Controls expiration of access, refresh, and session tokens.

Examples:

```
AUTHARA_ACCESS_TOKEN_TTL_MINUTES
AUTHARA_SESSION_TTL_DAYS
AUTHARA_REFRESH_TOKEN_TTL_DAYS
```

---

### JWT

Controls token signing and verification.

See: [JWT](jwt.md)

Examples:

```
AUTHARA_JWT_ISSUER
AUTHARA_JWT_KEYS
AUTHARA_JWT_ACTIVE_KEY_ID
```

---

### OAuth

Configures external identity providers.

See: [OAuth](oauth.md)

Examples:

```
AUTHARA_OAUTH_PROVIDERS
AUTHARA_OAUTH_GOOGLE_CLIENT_ID
```

---

### Challenge & verification

Controls email verification and challenge flows.

See: [Challenge](challenge.md)

Examples:

```
AUTHARA_CHALLENGE_ENABLED
AUTHARA_CHALLENGE_TTL
AUTHARA_CHALLENGE_MAX_ATTEMPTS
```

---

### Email

Controls email delivery via SMTP or other providers.

See: [Email](email.md)

Examples:

```
AUTHARA_EMAIL_PROVIDER
AUTHARA_EMAIL_FROM
AUTHARA_EMAIL_SMTP_HOST
```

---

### Email worker

Controls background processing of email jobs.

See: [Email](email.md)

Examples:

```
AUTHARA_EMAIL_WORKER_COUNT
AUTHARA_EMAIL_WORKER_POLL_INTERVAL
```

---

### Email cleanup

Controls retention of email job records.

See: [Email](email.md)

Examples:

```
AUTHARA_EMAIL_CLEANUP_SENT_AFTER
AUTHARA_EMAIL_CLEANUP_FAILED_AFTER
```

---

### Rate limiting

Protects login and signup endpoints.

See: [Rate Limiting](rate-limiting.md)

Examples:

```
AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT
AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT
```

---

### Webhooks

Configures event delivery to external systems.

Examples:

```
AUTHARA_WEBHOOK_URL
AUTHARA_WEBHOOK_SECRET
```

---

### Database connection pooling

Controls database performance and concurrency.

See: [Database Connection](database-connection.md)

Examples:

```
AUTHARA_DB_MAX_OPEN_CONNS
AUTHARA_DB_MAX_IDLE_CONNS
```

---

### Access policy

Restricts access via email allowlists.

See: [Access Policy](access-policy.md)

Examples:

```
AUTHARA_ACCESS_POLICY_ALLOWLIST_ENABLED
```

---

## Configuration reference

For the complete list of variables:

See: [Reference](reference.md)
