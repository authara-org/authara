# Configuration Overview

Authara bootstrap and infrastructure configuration is supplied through
**environment variables**. An explicitly registered subset of product and
operational policy is hybrid and can be overridden by an operator when its
environment variable is absent.

Configuration is typically provided via:

- `.env` files
- container environments
- orchestration platforms (Kubernetes, ECS, etc.)

Authara does **not load configuration files directly**.

See [Operator runtime settings](../operations/runtime-settings.md) for
precedence, environment locks, reset behavior, and propagation timing.

---

## Configuration categories

Authara configuration is organized into the following areas:

### Runtime

Controls environment and logging behavior.

Examples:

```
APP_ENV
LOG_LEVEL
AUTHARA_DEFAULT_RETURN_TO
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

Core and the migrations image share these TLS settings. See
`configuration/database-connection.md` for the supported modes and CA mounting.

---

### Cache

Configures the optional cache backend.

→ See: `configuration/cache.md`

Examples:

```
AUTHARA_CACHE_PROVIDER
AUTHARA_ACCESS_TOKEN_REVOCATION_MODE
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

`AUTHARA_USERNAME_LOGIN_ENABLED` controls whether password login accepts a
username in addition to an email address. It defaults to `false`.

`AUTHARA_PASSKEY_CLONE_RESPONSE` controls the response to a newly detected
passkey sign-counter anomaly: `alert` records the event and allows the request,
`restrict` also disables that passkey and denies the request, and
`restrict_and_revoke` additionally revokes all sessions. It defaults to
`alert`. `AUTHARA_PASSKEY_CLONE_NOTIFY_USER` defaults to `true` and queues a
generic security email without credential IDs, public keys, authenticator IDs,
or counter values.

`AUTHARA_PASSWORD_MIN_LENGTH` controls the minimum Unicode-character length for
new passwords (8–128, default 8; 15 or more is recommended in production).
Authara does not impose uppercase, lowercase, digit, or symbol composition
rules. This startup setting requires a restart; existing passwords continue to
authenticate when the minimum changes.

After a successful password proof, Authara compares the stored Argon2id costs,
salt length, and derived-key length with the current parameters. An outdated
hash is replaced before authentication completes. The write uses the exact old
hash as a compare-and-swap condition, so a concurrent password change or reset
cannot be overwritten. This maintenance update does not revoke sessions, send
password-change mail, or publish a user-change event. If a required upgrade
cannot be safely stored, the authentication request fails without changing the
credential.

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

Access-token lifetime accepts 1 through 1440 minutes. Expiry-only revocation
mode further limits it to 10 minutes.

`AUTHARA_RECENT_AUTHENTICATION_ENABLED` defaults to `true`. Set it to `false`
to let any active session perform sensitive account and administrative
mutations without step-up authentication. This weakens protection against
stolen or unattended sessions and takes effect immediately.

`AUTHARA_RECENT_AUTHENTICATION_WINDOW` defaults to `10m` and controls how long
a password, passkey, or linked-provider proof authorizes sensitive mutations.
It accepts Go durations from `1m` through `1h`.

### Organizations

Controls organization mode and invitation expiry.

`AUTHARA_ORG_MODE` is intended to be chosen before first production use.
Changing it after users or organizations exist is currently unsupported.

Modes:
- `personal`: direct signup creates a hidden personal org; invitations are disabled.
- `single`: direct signup creates one team org; invite signup joins the invited org.
- `multi`: direct signup creates a personal org; invite signup also joins the invited org.

Example:

```
AUTHARA_ORG_MODE
AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED
AUTHARA_ORGANIZATION_INVITATION_TTL
AUTHARA_INTERNAL_API_TOKEN
```

`AUTHARA_INTERNAL_API_TOKEN` is required for calls to `/auth/internal/v1` and
must be at least 24 characters when `APP_ENV=prod`.
`AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED` allows authenticated clients
to manage non-capacity organization data through matching `/auth/api/v1`
routes. Organization creation, invitation creation, and invitation resend remain
internal-only for backend billing and seat-limit checks and remain available
through bearer authentication in either mode.

---

### JWT

Controls token signing and verification.

→ See: `configuration/jwt.md`

Examples:

```
AUTHARA_JWT_ISSUER
AUTHARA_JWT_KEYS
AUTHARA_JWT_ACTIVE_KEY_ID
```

---

### OAuth

Configures external identity providers.

→ See: `configuration/oauth.md`

Examples:

```
AUTHARA_OAUTH_PROVIDERS
AUTHARA_OAUTH_GOOGLE_CLIENT_ID
```

---

### Challenge & verification

Controls email-code verification flows. Authentication challenges for
sensitive mutations are governed by the recent-authentication window instead.

→ See: `configuration/challenge.md`

Examples:

```
AUTHARA_CHALLENGE_ENABLED
AUTHARA_CHALLENGE_TTL
AUTHARA_CHALLENGE_MAX_ATTEMPTS
```

---

### Email

Controls email delivery via SMTP or other providers.

→ See: `configuration/email.md`

Examples:

```
AUTHARA_EMAIL_PROVIDER
AUTHARA_EMAIL_FROM
AUTHARA_EMAIL_SMTP_HOST
```

---

### Email worker

Controls background processing of email jobs.

→ See: `configuration/email.md`

Examples:

```
AUTHARA_EMAIL_WORKER_COUNT
AUTHARA_EMAIL_WORKER_POLL_INTERVAL
```

---

### Email cleanup

Controls retention of email job records.

→ See: `configuration/email.md`

Examples:

```
AUTHARA_EMAIL_CLEANUP_SENT_AFTER
AUTHARA_EMAIL_CLEANUP_FAILED_AFTER
AUTHARA_EMAIL_CLEANUP_INTERVAL
```

---

### Rate limiting

Protects login, signup, and passkey challenge endpoints.

→ See: `configuration/rate-limiting.md`

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

→ See: `configuration/database-connection.md`

Examples:

```
AUTHARA_DB_MAX_OPEN_CONNS
AUTHARA_DB_MAX_IDLE_CONNS
```

---

### Access policy

Restricts access via email allowlists.

→ See: `configuration/access-policy.md`

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

Controls the persisted event allowlist and independent retention period.

Examples:

```
AUTHARA_SECURITY_EVENT_ENABLED_EVENTS
AUTHARA_SECURITY_EVENT_RETENTION_DAYS
AUTHARA_SECURITY_EVENT_CLEANUP_INTERVAL
```

---

## Configuration reference

For the complete list of variables:

→ `configuration/reference.md`
