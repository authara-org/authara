package meta

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/authara-org/authara/internal/session/token"
)

const JWKSCacheControl = "public, max-age=300, must-revalidate"

// JWKS publishes only the public portion of every configured access-token key.
// The response is immutable for the lifetime of the process because signing
// keys are startup configuration.
func JWKS(keys *token.KeySet) http.Handler {
	if keys == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "JWKS unavailable", http.StatusServiceUnavailable)
		})
	}

	body, err := json.Marshal(keys.JWKS())
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", JWKSCacheControl)
		w.Header().Set("Content-Type", "application/jwk-set+json")
		w.Header().Set("ETag", etag)
		if etagMatches(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}

func etagMatches(header, etag string) bool {
	for value := range strings.SplitSeq(header, ",") {
		value = strings.TrimSpace(value)
		if value == "*" || value == etag || strings.TrimPrefix(value, "W/") == etag {
			return true
		}
	}
	return false
}
