package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/ratelimiter"
)

func TestRateLimitResultDistinguishesLimitsFromBackendErrors(t *testing.T) {
	tests := []struct {
		name        string
		allowed     bool
		err         error
		wantCode    response.ErrorCode
		wantMessage string
		wantOK      bool
	}{
		{name: "allowed", allowed: true, wantOK: true},
		{name: "typed limit", err: &ratelimiter.RateLimitedError{RetryAfter: time.Second}, wantCode: response.CodeRateLimited, wantMessage: "slow down"},
		{name: "false without error", wantCode: response.CodeRateLimited, wantMessage: "slow down"},
		{name: "backend error", err: errors.New("redis unavailable"), wantCode: response.CodeInternalError, wantMessage: "Authentication service unavailable."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output strings.Builder
			h := &APIHandler{Logger: slog.New(slog.NewTextHandler(&output, nil))}
			code, message, ok := h.rateLimitResult(test.allowed, test.err, "slow down")
			if code != test.wantCode || message != test.wantMessage || ok != test.wantOK {
				t.Fatalf("result = (%q, %q, %t), want (%q, %q, %t)", code, message, ok, test.wantCode, test.wantMessage, test.wantOK)
			}
			if test.name == "backend error" && (!strings.Contains(output.String(), "rate limiter unavailable") || !strings.Contains(output.String(), "redis unavailable")) {
				t.Fatalf("logs = %q, want limiter and root error", output.String())
			}
		})
	}
}

func TestLoginWithPasswordReportsLimiterBackendFailureAsInternalError(t *testing.T) {
	tests := []struct {
		name       string
		counter    testRateLimitCounter
		wantStatus int
		wantCode   string
	}{
		{name: "backend unavailable", counter: testRateLimitCounter{err: errors.New("redis unavailable")}, wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
		{name: "limit exceeded", counter: testRateLimitCounter{count: 2}, wantStatus: http.StatusTooManyRequests, wantCode: "rate_limited"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &APIHandler{
				Limiter: ratelimiter.NewCacheLimiter(test.counter, ratelimiter.LimiterConfig{
					LoginIPLimit: 1, LoginEmailLimit: 1,
				}),
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}
			req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/login", nil)
			resp, err := h.LoginWithPassword(contractCtx(context.Background(), req), contract.LoginWithPasswordRequestObject{
				Body: passwordLoginRequest("person@example.com", "password123"),
			})
			if err != nil {
				t.Fatalf("LoginWithPassword failed: %v", err)
			}
			rr := httptest.NewRecorder()
			writeContractResponse(t, rr, resp)
			if rr.Code != test.wantStatus || !strings.Contains(rr.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("response = %d %s, want %d with %s", rr.Code, rr.Body.String(), test.wantStatus, test.wantCode)
			}
		})
	}
}

type testRateLimitCounter struct {
	count int64
	err   error
}

func (c testRateLimitCounter) Increment(context.Context, string, time.Duration) (int64, time.Duration, error) {
	return c.count, time.Minute, c.err
}
