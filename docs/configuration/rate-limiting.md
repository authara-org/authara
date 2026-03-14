# rate limiting

authara includes built-in rate limiting to protect against:

- brute-force login attempts
- automated signup abuse

limits are applied per:

- ip address
- email address

---

## login limits

### authara_rate_limit_login_ip_limit

maximum login attempts per ip.

default:

```
5
```

---

### authara_rate_limit_login_ip_window

time window for ip-based login attempts.

default:

```
1m
```

---

### authara_rate_limit_login_email_limit

maximum login attempts per email.

default:

```
10
```

---

### authara_rate_limit_login_email_window

email login attempt window.

default:

```
1h
```

---

## signup limits

### authara_rate_limit_signup_ip_limit

maximum signup attempts per ip.

default:

```
3
```

---

### authara_rate_limit_signup_ip_window

signup ip window.

default:

```
1h
```

---

### authara_rate_limit_signup_email_limit

maximum signup attempts per email.

default:

```
3
```

---

### authara_rate_limit_signup_email_window

signup email window.

default:

```
24h
```

---

## safety limits

### authara_rate_limit_max_entries

maximum number of rate limit keys stored in memory.

default:

```
50000
```

this acts as a safety valve against memory exhaustion.

---

## multi-instance deployments

current rate limiting is **in-memory per instance**.

in multi-instance deployments limits are not shared between instances.

future versions may support shared rate limiting via redis.
