# Cache

Authara can use an optional cache backend for shared runtime state.

When Redis is configured, rate limiting uses Redis-backed counters and revoked
access tokens are rejected immediately across Authara instances. Revocation
entries expire automatically after the access-token TTL. Authara stores token
hashes, never bearer-token values.

Access-token revocation checks fail closed: when Redis is configured but
unavailable, authenticated requests are rejected until the cache recovers.

Core's Redis revocation key templates are defined by the stable machine-readable
contract in `contract/access-token-revocations.json`. Server-side SDKs that
perform revocation checks synchronize and test their keys against it.

With the `noop` cache, rate limiting stays in memory per instance and access
tokens remain valid until their normal expiry after a session, user, or
organization membership is revoked.

---

See also: [Rate Limiting](rate-limiting.md)

---

## AUTHARA_CACHE_PROVIDER

Cache backend.

Supported values:

- `noop`
- `redis`

Default:

```
noop
```

---

## Redis

Used when `AUTHARA_CACHE_PROVIDER=redis`.

### AUTHARA_REDIS_HOST

Redis host.

Default:

```
localhost
```

### AUTHARA_REDIS_PORT

Redis port.

Default:

```
6379
```

### AUTHARA_REDIS_PASSWORD

Redis password.

Default: empty.

### AUTHARA_REDIS_DB

Redis database number.

Default:

```
0
```
