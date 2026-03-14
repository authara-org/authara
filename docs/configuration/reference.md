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

---

## Advanced configuration

Additional configuration areas:

- JWT signing keys
- OAuth providers
- rate limiting

These are documented in their dedicated sections.
