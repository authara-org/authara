package ui

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/ratelimiter"
)

func TestRateLimitResultDistinguishesLimitsFromBackendErrors(t *testing.T) {
	h := &UIHandler{}
	tests := []struct {
		name        string
		allowed     bool
		err         error
		wantStatus  int
		wantMessage string
		wantOK      bool
	}{
		{name: "allowed", allowed: true, wantOK: true},
		{name: "typed limit", err: &ratelimiter.RateLimitedError{RetryAfter: time.Second}, wantStatus: http.StatusTooManyRequests, wantMessage: "slow down"},
		{name: "false without error", wantStatus: http.StatusTooManyRequests, wantMessage: "slow down"},
		{name: "backend error", err: errors.New("redis unavailable"), wantStatus: http.StatusInternalServerError, wantMessage: "Authentication service unavailable."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, message, ok := h.rateLimitResult(test.allowed, test.err, "slow down")
			if status != test.wantStatus || message != test.wantMessage || ok != test.wantOK {
				t.Fatalf("result = (%d, %q, %t), want (%d, %q, %t)", status, message, ok, test.wantStatus, test.wantMessage, test.wantOK)
			}
		})
	}
}

func TestRateLimiterBackendFailureReturnsInternalErrorFromUIEndpoints(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	limiter := ratelimiter.NewCacheLimiter(uiRateLimitCounter{err: errors.New("redis unavailable")}, ratelimiter.LimiterConfig{
		LoginIPLimit:        1,
		LoginEmailLimit:     1,
		PasskeyLoginIPLimit: 1,
	})

	t.Run("JSON passkey endpoint", func(t *testing.T) {
		h := &UIHandler{Passkeys: &passkey.Service{}, Limiter: limiter, Logger: logger}
		rr := httptest.NewRecorder()
		h.PasskeyAuthenticateOptionsPost(rr, httptest.NewRequest(http.MethodPost, "/auth/passkeys/authenticate/options", nil))

		if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), `"code":"internal_error"`) {
			t.Fatalf("response = %d %s, want 500 internal_error", rr.Code, rr.Body.String())
		}
	})

	t.Run("HTML login endpoint", func(t *testing.T) {
		h := &UIHandler{
			Limiter: limiter,
			Logger:  logger,
			Render:  render.New(render.Assets{}, false),
		}
		form := url.Values{"email": {"person@example.com"}, "password": {"password123"}}
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		h.LoginPost(rr, req)

		if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "Authentication service unavailable.") {
			t.Fatalf("response = %d %s, want generic 500 error", rr.Code, rr.Body.String())
		}
	})
}

type uiRateLimitCounter struct {
	err error
}

func (c uiRateLimitCounter) Increment(context.Context, string, time.Duration) (int64, time.Duration, error) {
	return 0, time.Minute, c.err
}
