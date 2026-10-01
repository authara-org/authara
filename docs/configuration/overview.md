# Configuration Overview

Authara bootstrap and infrastructure configuration is supplied through
**environment variables**. An explicitly registered subset of product and
operational policy can also be overridden at runtime by an Authara operator.

Configuration is typically provided via:

- `.env` files
- container environments
- orchestration platforms (Kubernetes, ECS, etc.)

Authara does **not load configuration files directly**.

See [Operator runtime settings](../operations/runtime-settings.md) for the
hybrid precedence and locking model. Environment-only settings and all secrets
remain deployment-controlled.

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
POSTGRESQL_SSL_MODE
POSTGRESQL_SSL_ROOT_CERT
```

Core and the migrations image use the same TLS values. See
`configuration/database-connection.md`.

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

### Authentication

Controls optional authentication behavior and the policy applied when a
password is created or replaced. Username login is disabled by default; email
login remains available in either mode. Passwords are measured in Unicode
characters and allow passphrases without composition rules. A minimum of 15 is
recommended for production.

Example:

```
AUTHARA_USERNAME_LOGIN_ENABLED
AUTHARA_EMAIL_VERIFICATION_REQUIRED
AUTHARA_PASSKEY_CLONE_RESPONSE
AUTHARA_PASSKEY_CLONE_NOTIFY_USER
AUTHARA_PASSWORD_MIN_LENGTH
```

`AUTHARA_EMAIL_VERIFICATION_REQUIRED` defaults to `false`. Enabling it requires
challenge delivery and a configured email provider. Existing unverified users
are signed out on their next authenticated request and routed through the email
verification page, where they may replace and verify an obsolete address.

---

### Token lifetimes

Controls expiration of access, refresh, and session tokens.

Examples:

```
AUTHARA_ACCESS_TOKEN_TTL_MINUTES
AUTHARA_SESSION_TTL_DAYS
AUTHARA_REFRESH_TOKEN_TTL_DAYS
AUTHARA_REFRESH_TOKEN_ROTATION_INTERVAL
AUTHARA_RECENT_AUTHENTICATION_ENABLED
AUTHARA_RECENT_AUTHENTICATION_WINDOW
AUTHARA_SESSION_CLEANUP_INTERVAL
```

---

### Organizations

Controls organization mode, invitation expiry, and the server-to-server token for internal organization APIs.

`AUTHARA_ORG_MODE` is intended to be chosen before first production use.
Changing it after users or organizations exist is currently unsupported.

Modes:
- `personal`: direct signup creates a hidden personal org; invitations are disabled.
- `single`: direct signup creates one team org; invite signup joins the invited org.
- `multi`: direct signup creates a personal org; invite signup also joins the invited org.

Examples:

```
AUTHARA_ORG_MODE
AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED
AUTHARA_ORGANIZATION_INVITATION_TTL
AUTHARA_INTERNAL_API_TOKEN
```

`AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED` allows authenticated clients
to manage non-capacity organization data directly through Authara.
Organization creation, invitation creation, and invitation resend remain
internal-only for backend billing and seat-limit checks.

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

Controls email-code verification flows. Authentication challenges for
sensitive mutations are governed by the recent-authentication window instead.

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
AUTHARA_EMAIL_CLEANUP_INTERVAL
```

---

### Rate limiting

Protects login, signup, and passkey challenge endpoints.

See: [Rate Limiting](rate-limiting.md)

Examples:

```
AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT
AUTHARA_RATE_LIMIT_PASSKEY_LOGIN_IP_LIMIT
AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT
```

---

### Webhooks

Configures event delivery to external systems.

Examples:

```
AUTHARA_WEBHOOK_URL
AUTHARA_WEBHOOK_SECRET
AUTHARA_WEBHOOK_WORKER_COUNT
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

### Admin audit

Controls retention for admin audit events.

Examples:

```
AUTHARA_ADMIN_AUDIT_RETENTION_DAYS
AUTHARA_ADMIN_AUDIT_CLEANUP_INTERVAL
```

---

### Security events

Controls which authentication and credential events are persisted and how long they are retained.

Examples:

```
AUTHARA_SECURITY_EVENT_ENABLED_EVENTS
AUTHARA_SECURITY_EVENT_RETENTION_DAYS
AUTHARA_SECURITY_EVENT_CLEANUP_INTERVAL
```

---

## Configuration reference

For the complete list of variables:

See: [Reference](reference.md)
