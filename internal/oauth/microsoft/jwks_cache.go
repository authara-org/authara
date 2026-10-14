package microsoft

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	jwksResponseTTL      = time.Minute
	jwksFetchWindow      = time.Minute
	jwksFetchLimit       = 64
	maxJWKSResponseBytes = 1 << 20
)

var errJWKSFetchLimited = errors.New("microsoft JWKS refresh limit exceeded")

type cachedJWKSResponse struct {
	status     string
	statusCode int
	header     http.Header
	body       []byte
	expiresAt  time.Time
}

type jwksCachingTransport struct {
	base http.RoundTripper
	now  func() time.Time

	mu           sync.Mutex
	responses    map[string]cachedJWKSResponse
	responseFIFO []string
	windowStart  time.Time
	fetches      int
}

func newJWKSHTTPClient(client *http.Client) *http.Client {
	clone := *client
	base := clone.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone.Transport = &jwksCachingTransport{
		base: base, now: time.Now, responses: make(map[string]cachedJWKSResponse),
	}
	return &clone
}

func (t *jwksCachingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Method != http.MethodGet {
		return t.base.RoundTrip(request)
	}
	now := t.now()
	key := request.URL.String()

	t.mu.Lock()
	if cached, ok := t.responses[key]; ok && now.Before(cached.expiresAt) {
		t.mu.Unlock()
		return cached.response(request), nil
	}
	if t.windowStart.IsZero() || !now.Before(t.windowStart.Add(jwksFetchWindow)) {
		t.windowStart = now
		t.fetches = 0
	}
	if t.fetches >= jwksFetchLimit {
		t.mu.Unlock()
		return nil, errJWKSFetchLimited
	}
	t.fetches++
	t.mu.Unlock()

	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxJWKSResponseBytes+1))
	_ = response.Body.Close()
	if err != nil {
		return nil, err
	}
	if len(body) > maxJWKSResponseBytes {
		return nil, fmt.Errorf("microsoft JWKS response exceeds %d bytes", maxJWKSResponseBytes)
	}

	cached := cachedJWKSResponse{
		status: response.Status, statusCode: response.StatusCode,
		header: response.Header.Clone(), body: body, expiresAt: now.Add(jwksResponseTTL),
	}
	if response.StatusCode == http.StatusOK ||
		(response.StatusCode >= http.StatusBadRequest && response.StatusCode < http.StatusInternalServerError && response.StatusCode != http.StatusTooManyRequests) {
		t.store(key, cached)
	}
	return cached.response(request), nil
}

func (t *jwksCachingTransport) store(key string, response cachedJWKSResponse) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.responses[key]; !exists {
		if len(t.responseFIFO) >= maxTenantKeySets {
			delete(t.responses, t.responseFIFO[0])
			t.responseFIFO = t.responseFIFO[1:]
		}
		t.responseFIFO = append(t.responseFIFO, key)
	}
	t.responses[key] = response
}

func (r cachedJWKSResponse) response(request *http.Request) *http.Response {
	return &http.Response{
		Status: r.status, StatusCode: r.statusCode, Header: r.header.Clone(),
		Body: io.NopCloser(bytes.NewReader(r.body)), ContentLength: int64(len(r.body)),
		Request: request,
	}
}
