# Cache

Authara can use an optional cache backend for shared runtime state. Access-token
revocation is a separate, explicit security choice so a small deployment is not
forced to operate Redis without acknowledging the trade-off.

Use one of these homogeneous deployment profiles:

- `immediate`: requires `AUTHARA_CACHE_PROVIDER=redis`. Rate limiting uses
  shared counters and revoked access tokens are rejected across Authara
  instances. Revocation entries expire automatically; Authara stores token
  hashes, never bearer-token values. Access tokens are capped at 24 hours so a
  marker still covers tokens issued before a runtime lifetime reduction.
- `expiry`: requires `AUTHARA_CACHE_PROVIDER=noop`. Refresh tokens and database
  sessions are revoked immediately, but an already-issued access JWT can remain
  valid until expiry. Authara caps its lifetime at 10 minutes in both startup
  and runtime configuration.

When the mode is omitted, Redis implies `immediate` and a development noop
cache implies `expiry`. Production with noop does not start until
`AUTHARA_ACCESS_TOKEN_REVOCATION_MODE=expiry` explicitly acknowledges the
residual risk.

In immediate mode, access-token revocation checks fail closed: when Redis is
unavailable, protected requests return `503` until it recovers. Mutations such
as logout, account disable, credential change, and membership or session
revocation report failure instead of committing a database-only revocation.
Refresh-token reuse detection is the exception: the compromised database
session is revoked even if the Redis marker cannot be written, and the request
still fails.

Authentication endpoints also fail closed when Redis-backed rate limiting is
unavailable. The API returns `500 internal_error` rather than incorrectly
reporting `429 rate_limited`; HTML flows show a generic authentication-service
error. Readiness returns `503` while Redis cannot be reached.

Core's Redis revocation key templates are defined by the stable machine-readable
contract in `contract/access-token-revocations.json`. Server-side SDKs that
perform revocation checks synchronize and test their keys against it.

Do not mix immediate and expiry replicas: an expiry replica cannot observe a
Redis revocation written by an immediate replica. When changing modes, drain
old replicas. After any reported revocation-marker write failure, or before
relying on a restored or replaced Redis store, either keep protected traffic
unavailable for the longest access-token lifetime that was previously issued,
or rotate the JWT signing key and remove the old verification key. Configure
Redis persistence and an eviction policy appropriate for security state;
losing revocation keys has the same recovery requirement.

Before upgrading a deployment that previously issued access tokens with a
lifetime above 24 hours, either wait out the longest issued lifetime with
protected traffic fenced or rotate the signing key and remove the old
verification key. Lowering the configured lifetime does not shorten JWTs that
already exist.

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

## AUTHARA_ACCESS_TOKEN_REVOCATION_MODE

Access-token invalidation guarantee.

Supported values:

- `immediate` — Redis-backed, fail-closed revocation
- `expiry` — no online revocation, with a maximum 10-minute access-token TTL

There is no unconditional default because production/noop must opt into its
residual risk. Redis infers `immediate`; dev/noop infers `expiry`.

---

## Redis

Used when `AUTHARA_CACHE_PROVIDER=redis`.

The current Redis transport is unencrypted. Core emits a startup warning when
Redis is selected; place Redis on a trusted private network and prevent direct
or shared-network access until TLS transport support is configured.

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
