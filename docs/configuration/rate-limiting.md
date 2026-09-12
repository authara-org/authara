# Rate Limiting

Authara includes built-in rate limiting to protect against:

- brute-force login attempts
- automated signup abuse
- unauthenticated passkey challenge creation

Limits are applied per:

- IP address
- email address (or username when username login is enabled)

---

See also: [Configuration Reference](reference.md)

---

All rate-limit variables are also available on the operator runtime-settings
page. Leave an environment variable unset to use the built-in default and keep
that setting live-editable. Setting one in the environment pins and locks only
that value until Core is restarted without it.

---

## Login limits

### AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT

Maximum login attempts per IP.

Default:

```
5
```

---

### AUTHARA_RATE_LIMIT_LOGIN_IP_WINDOW

Time window for IP-based login attempts.

Default:

```
1m
```

---

### AUTHARA_RATE_LIMIT_LOGIN_EMAIL_LIMIT

Maximum login attempts per submitted email address or enabled username. The
environment variable retains `EMAIL` in its name for backwards compatibility.

Default:

```
10
```

---

### AUTHARA_RATE_LIMIT_LOGIN_EMAIL_WINDOW

Time window for email- or enabled-username-based login attempts.

Default:

```
1h
```

---

## Signup limits

### AUTHARA_RATE_LIMIT_SIGNUP_IP_LIMIT

Maximum signup attempts per IP.

Default:

```
3
```

---

### AUTHARA_RATE_LIMIT_SIGNUP_IP_WINDOW

Time window for IP-based signup attempts.

Default:

```
1h
```

---

### AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT

Maximum signup attempts per email.

Default:

```
3
```

---

### AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_WINDOW

Time window for email-based signup attempts.

Default:

```
24h
```

---

## Passkey login limits

### AUTHARA_RATE_LIMIT_PASSKEY_LOGIN_IP_LIMIT

Maximum passkey login option requests and finish requests per IP. Options and
finishes use independent counters, so one complete ceremony consumes one slot
from each counter.

Default:

```
30
```

---

### AUTHARA_RATE_LIMIT_PASSKEY_LOGIN_IP_WINDOW

Time window for the independent IP-based passkey login option and finish
counters.

Default:

```
10m
```

---

## Password-reset limits

Password-reset requests use independent IP and email buckets configured by:

- `AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_LIMIT` (default `5`)
- `AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_WINDOW` (default `1h`)
- `AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_LIMIT` (default `3`)
- `AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_WINDOW` (default `24h`)

## Challenge limits

Challenge verification and resend requests use independent IP buckets:

- `AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_LIMIT` (default `30`)
- `AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_WINDOW` (default `10m`)
- `AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_LIMIT` (default `10`)
- `AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_WINDOW` (default `1h`)

Changing a threshold applies to the next limiter check. Changing a window does
not rewrite an existing bucket's reset deadline; it applies when the next
bucket is created.

---

## Safety limits

### AUTHARA_RATE_LIMIT_CLEANUP_EVERY

Number of in-memory limiter calls between expired-entry sweeps. The default is
`200`. This setting is unused by the Redis limiter.

### AUTHARA_RATE_LIMIT_MAX_ENTRIES

Maximum number of rate limit keys stored in memory.

Default:

```
50000
```

This acts as a safety valve against memory exhaustion.

Both safety settings are live-editable for the in-memory limiter.

---

## Multi-instance deployments

By default, rate limiting is **in-memory per instance**.

In multi-instance deployments, limits are **not shared** between instances.

Set `AUTHARA_CACHE_PROVIDER=redis` to use Redis-backed counters and share
rate limits across Authara instances.
