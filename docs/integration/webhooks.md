# Webhooks

Authara can notify your application about **user lifecycle events** via webhooks.

This allows your application to stay in sync with Authara without polling or tightly coupling to internal state.

---

# Overview

When configured, Authara sends HTTP POST requests to your webhook endpoint.

Supported events:

- `user.created`
- `user.deleted`

---

# Configuration

Webhooks are configured via environment variables.

## Required

```env
AUTHARA_WEBHOOK_URL=https://app.example.com/webhooks/authara
AUTHARA_WEBHOOK_SECRET=your-secret
```

## Optional

```env
AUTHARA_WEBHOOK_ENABLED_EVENTS=user.created,user.deleted
AUTHARA_WEBHOOK_TIMEOUT=5s
```

---

## Variables

### `AUTHARA_WEBHOOK_URL`

The endpoint that receives webhook events.

Must include scheme (`http://` or `https://`).

---

### `AUTHARA_WEBHOOK_SECRET`

Shared secret used to sign webhook requests.

Your application must verify incoming requests using this secret.

---

### `AUTHARA_WEBHOOK_ENABLED_EVENTS`

Comma-separated list of events to send.

Example:

```env
AUTHARA_WEBHOOK_ENABLED_EVENTS=user.created,user.deleted
```

**Default behavior:**

If unset, **all supported events are sent**.

---

### `AUTHARA_WEBHOOK_TIMEOUT`

HTTP timeout for webhook delivery.

Default:

```env
5s
```

---

# Event Delivery

Authara sends webhook events as HTTP POST requests.

## Request

```
POST /your-endpoint
Content-Type: application/json
X-Authara-Event: user.created
X-Authara-Delivery: evt_123
X-Authara-Signature: sha256=...
```

---

## Body

```json
{
  "id": "evt_123",
  "type": "user.created",
  "created_at": "2026-03-20T12:00:00Z",
  "data": {
    "user_id": "uuid"
  }
}
```

---

# Signature Verification

Each request is signed using HMAC-SHA256.

Header:

```
X-Authara-Signature: sha256=<hex>
```

Computed as:

```
HMAC_SHA256(secret, request_body)
```

---

## Example (Go)

```go
func verifySignature(secret string, body []byte, header string) bool {
	expected := webhook.Sign(secret, body)
	return hmac.Equal([]byte(expected), []byte(header))
}
```

Always verify signatures before processing webhook events.

---

# Delivery Semantics

- Webhooks are sent **after the action succeeds**
- Delivery is **best-effort**
- Failed deliveries are **not retried** (current version)

This means:

- your endpoint must be reliable
- your handler should be idempotent

---

# Idempotency

Each event includes a unique ID:

```json
"id": "evt_123"
```

Your application should:

- track processed event IDs
- ignore duplicates

---

# Example Handler

```go
http.HandleFunc("/webhooks/authara", func(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Authara-Signature")

	if !verifySignature(os.Getenv("AUTHARA_WEBHOOK_SECRET"), body, signature) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var evt struct {
		ID    string `json:"id"`
		Event string `json:"type"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &evt); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	switch evt.Event {
	case "user.created":
		// handle user creation
	case "user.deleted":
		// handle user deletion
	}

	w.WriteHeader(http.StatusNoContent)
})
```

---

# Security

- Always verify webhook signatures
- Use HTTPS in production
- Treat webhook data as untrusted input

---

# Summary

Webhooks allow Authara to notify your application about important events.

They are:

- simple to configure
- easy to integrate
- essential for keeping your application in sync
