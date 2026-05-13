# Rate Limiting

Authara includes built-in rate limiting to protect against:

- brute-force login attempts
- automated signup abuse
- unauthenticated passkey challenge creation

Limits are applied per:

- IP address
- email address

---

See also: [Configuration Reference](reference.md)

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

Maximum login attempts per email.

Default:

```
10
```

---

### AUTHARA_RATE_LIMIT_LOGIN_EMAIL_WINDOW

Time window for email-based login attempts.

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

Maximum passkey login option requests per IP.

Default:

```
30
```

---

### AUTHARA_RATE_LIMIT_PASSKEY_LOGIN_IP_WINDOW

Time window for IP-based passkey login option requests.

Default:

```
10m
```

---

## Safety limits

### AUTHARA_RATE_LIMIT_MAX_ENTRIES

Maximum number of rate limit keys stored in memory.

Default:

```
50000
```

This acts as a safety valve against memory exhaustion.

---

## Multi-instance deployments

By default, rate limiting is **in-memory per instance**.

In multi-instance deployments, limits are **not shared** between instances.

Set `AUTHARA_CACHE_PROVIDER=redis` to use Redis-backed counters and share
rate limits across Authara instances.
