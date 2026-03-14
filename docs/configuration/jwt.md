# JWT Configuration

Authara issues **JSON Web Tokens (JWTs)** as short-lived access tokens.

JWTs are used to authenticate requests between the browser, the application, and Authara-protected APIs.

JWT configuration controls:

- the token issuer
- signing keys
- key rotation

---

# Overview

Authara signs access tokens using symmetric keys configured via environment variables.

Each key has a **key ID (`kid`)** which is embedded in the token header.  
This allows Authara to support **multiple active keys** for safe key rotation.

Relevant configuration variables:

```
AUTHARA_JWT_ISSUER
AUTHARA_JWT_ACTIVE_KEY_ID
AUTHARA_JWT_KEYS
```

---

# JWT Issuer

```
AUTHARA_JWT_ISSUER
```

Defines the `iss` claim included in all issued access tokens.

Example:

```
AUTHARA_JWT_ISSUER=authara
```

Applications validating tokens should verify that the `iss` claim matches the expected issuer.

---

# Active Signing Key

```
AUTHARA_JWT_ACTIVE_KEY_ID
```

Specifies the key ID used to sign **newly issued tokens**.

Example:

```
AUTHARA_JWT_ACTIVE_KEY_ID=key-2026-01
```

The key ID must exist in `AUTHARA_JWT_KEYS`.

---

# Signing Keys

```
AUTHARA_JWT_KEYS
```

Defines the set of signing keys available to Authara.

Format:

```
keyID:base64secret,keyID2:base64secret2
```

Example:

```
AUTHARA_JWT_KEYS=key-2026-01:VZp2u1sYz0g2nF2vY8q8dP7cZQpL5cRrXn0k7FZ0xkE=,key-2025-09:Qk8K6E3XrV6mF4T9yZcA2p9xYbDqZpM0JwH3uZ8sL1E=
```

Each entry contains:

```
keyID : base64-encoded secret
```

The key ID is included in the JWT header as the `kid` field.

---

# Key Rotation

Authara supports **safe key rotation** by allowing multiple signing keys.

The typical rotation process is:

1. Generate a new secret key.
2. Add the new key to `AUTHARA_JWT_KEYS`.
3. Update `AUTHARA_JWT_ACTIVE_KEY_ID` to the new key.
4. Restart Authara.

Example:

```
AUTHARA_JWT_ACTIVE_KEY_ID=key-2026-06

AUTHARA_JWT_KEYS=
  key-2026-06:NEW_SECRET,
  key-2026-01:OLD_SECRET
```

New tokens will be signed with the new key while older tokens continue to validate using the previous key.

Once all old tokens have expired, the previous key can be removed.

---

# Generating Keys

JWT secrets should be **cryptographically random**.

Example using OpenSSL:

```bash
openssl rand -base64 32
```

Example output:

```
VZp2u1sYz0g2nF2vY8q8dP7cZQpL5cRrXn0k7FZ0xkE=
```

---

# Security Recommendations

- Always generate keys using a secure random generator.
- Do not reuse secrets across environments.
- Rotate keys periodically.
- Store secrets in a secure secret management system when running in production.

---

# Token Lifetime

Access token lifetime is configured separately:

```
AUTHARA_ACCESS_TOKEN_TTL_MINUTES
```

default:

```
10
```

Access tokens are intentionally **short-lived**, while refresh tokens control the overall session lifetime.

---

# Summary

JWT configuration defines:

- the token issuer
- the active signing key
- the available signing keys for validation

Multiple keys allow **safe rotation without breaking existing sessions**.
