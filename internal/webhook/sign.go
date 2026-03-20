package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const (
	SignatureHeader    = "X-Authara-Signature"
	SignaturePrefix    = "sha256="
	SignatureAlgorithm = "hmac-sha256"
	SignatureFormat    = "sha256=<hex>"
)

func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return SignaturePrefix + hex.EncodeToString(mac.Sum(nil))
}
