# Configuration Overview

Authara is configured entirely through **environment variables**.

Configuration is typically provided via:

- `.env` files
- container environment variables
- orchestration platforms (Kubernetes, ECS, etc.)

Authara does **not load configuration files directly**.  
Environment variables are provided by the runtime environment.

---

## Configuration categories

Authara configuration is divided into several areas:

### Runtime

Controls runtime behavior such as environment and logging.

Examples:

```
APP_ENV
LOG_LEVEL
```

---

### Database

Defines the PostgreSQL connection used by Authara.

Examples:

```
POSTGRESQL_HOST
POSTGRESQL_DATABASE
POSTGRESQL_USERNAME
POSTGRESQL_PASSWORD
```

---

### JWT configuration

Controls how access tokens are signed and validated.

Examples:

```
AUTHARA_JWT_ISSUER
AUTHARA_JWT_KEYS
AUTHARA_JWT_ACTIVE_KEY_ID
```

---

### OAuth providers

Optional third-party authentication providers.

Examples:

```
AUTHARA_OAUTH_PROVIDERS
AUTHARA_OAUTH_GOOGLE_CLIENT_ID
```

---

### Rate limiting

Controls login and signup protection against brute-force attacks.

Examples:

```
AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT
AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT
```

---

## Configuration reference

For the complete list of available variables see:

```
Configuration → Reference
```
