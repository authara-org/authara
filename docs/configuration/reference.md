# Configuration Reference

The following environment variables control Authara behavior.

---

## Runtime

### APP_ENV

Runtime environment.

```
APP_ENV=dev
```

Allowed values:

```
dev
prod
```

---

### LOG_LEVEL

Overrides the default log level.

Defaults:

```
dev  → debug
prod → info
```

---

## Database

### POSTGRESQL_HOST

PostgreSQL host.

Example:

```
POSTGRESQL_HOST=postgres
```

---

### POSTGRESQL_PORT

PostgreSQL port.

Default:

```
5432
```

---

### POSTGRESQL_DATABASE

Database name.

```
POSTGRESQL_DATABASE=authara
```

---

### POSTGRESQL_USERNAME

Database user.

---

### POSTGRESQL_PASSWORD

Database password.

---

### POSTGRESQL_SCHEMA

Optional schema name.

Default:

```
authara
```

---

### POSTGRESQL_TIMEZONE

Database timezone.

Default:

```
UTC
```

---

### POSTGRESQL_LOG_SQL

Enables SQL query logging.

Default:

```
false
```

Recommended only for development.

---

## Public URL

### PUBLIC_URL

Externally visible base URL of the application.

Important:

- must **not** include `/auth`
- used for OAuth callbacks and redirects

Example:

```
PUBLIC_URL=http://localhost:3000
```

---

## Token lifetimes

### AUTHARA_ACCESS_TOKEN_TTL_MINUTES

Access token lifetime.

Default:

```
10 minutes
```

---

### AUTHARA_SESSION_TTL_DAYS

Absolute session lifetime.

Default:

```
60 days
```

---

### AUTHARA_REFRESH_TOKEN_TTL_DAYS

Refresh token lifetime.

Default:

```
14 days
```

## JWT

### AUTHARA_JWT_ISSUER

JWT issuer (`iss` claim).

Required: yes

---

### AUTHARA_JWT_ACTIVE_KEY_ID

Active signing key ID.

Required: yes

---

### AUTHARA_JWT_KEYS

Signing keys.

Format:

```
keyID:base64secret,keyID2:base64secret2
```

Required: yes

## OAuth

### AUTHARA_OAUTH_PROVIDERS

Comma-separated list of enabled OAuth providers.

Type: csv  
Default: empty

---

### AUTHARA_OAUTH_GOOGLE_CLIENT_ID

Google OAuth client ID.

Required if `google` is enabled

## Rate Limiting

### AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT
Default:
```
5
```

---

### AUTHARA_RATE_LIMIT_LOGIN_IP_WINDOW
Default:
```
1m
```

---

### AUTHARA_RATE_LIMIT_LOGIN_EMAIL_LIMIT
Default:
```
10
```

---

### AUTHARA_RATE_LIMIT_LOGIN_EMAIL_WINDOW
Default:
```
1h
```

---

### AUTHARA_RATE_LIMIT_SIGNUP_IP_LIMIT
Default:
```
3
```

---

### AUTHARA_RATE_LIMIT_SIGNUP_IP_WINDOW
Default:
```
1h
```

---

### AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT
Default:
```
3
```

---

### AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_WINDOW
Default:
```
24h
```

---

### AUTHARA_RATE_LIMIT_MAX_ENTRIES
Default:
```
50000
```

## Webhooks

### AUTHARA_WEBHOOK_URL

Webhook endpoint where Authara sends events.

Type: url  
Required: no

---

### AUTHARA_WEBHOOK_SECRET

Shared secret used to sign webhook requests.

Type: string  
Required: no

---

### AUTHARA_WEBHOOK_ENABLED_EVENTS

Comma-separated list of enabled events.

Type: csv  
Required: no  

Default: all events enabled

Allowed values:

- user.created
- user.deleted

---

### AUTHARA_WEBHOOK_TIMEOUT

HTTP timeout for webhook delivery.

Type: duration  
Default: 5s

---

## Database Connection Pool

### AUTHARA_DB_MAX_OPEN_CONNS
Default:
```
40
```

---

### AUTHARA_DB_MAX_IDLE_CONNS
Default:
```
20
```

---

### AUTHARA_DB_CONN_MAX_LIFETIME
Default:
```
30m
```

---

### AUTHARA_DB_CONN_MAX_IDLE_TIME
Default:
```
5m
```

## Access Policy

### AUTHARA_ACCESS_POLICY_ALLOWLIST_ENABLED

Enables email allowlist enforcement.

Type: boolean  
Default:
```
false
```

When enabled, only emails present in the `allowed_emails` table may register and authenticate.

## Advanced configuration

Authara includes additional configuration areas such as:

- JWT signing keys
- OAuth providers
- rate limiting
- database connection pooling
- access policy

All configuration variables are listed in this reference.

Detailed behavior and concepts for these areas are documented in their dedicated sections.
