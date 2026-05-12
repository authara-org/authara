# Cache

Authara can use an optional cache backend for shared runtime state.

When Redis is configured, rate limiting uses Redis-backed counters so limits
are shared across Authara instances. When cache is disabled, Authara uses a
noop cache and keeps rate limiting in memory per instance.

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
