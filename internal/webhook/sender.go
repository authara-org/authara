package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"
)

const (
	EventHeader       = "X-Authara-Event"
	DeliveryHeader    = "X-Authara-Delivery"
	DeliverySemantics = "best_effort"

	RetryReasonNetworkError = "network_error"
	RetryReasonHTTP429      = "http_429"
	RetryReasonHTTP5xx      = "http_5xx"
	RetryReasonOtherHTTP4xx = "other_http_4xx"
)

var RetryOn = []string{
	RetryReasonNetworkError,
	RetryReasonHTTP429,
	RetryReasonHTTP5xx,
}

var RetryNotOn = []string{
	RetryReasonOtherHTTP4xx,
}

type Publisher interface {
	Publish(ctx context.Context, evt Envelope) error
}

type Sender struct {
	URL      string
	Secret   string
	Client   *http.Client
	Attempts int
	Backoff  []time.Duration
}

func NewSender(url, secret string, client *http.Client) *Sender {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	return &Sender{
		URL:      url,
		Secret:   secret,
		Client:   client,
		Attempts: 3,
		Backoff:  []time.Duration{1 * time.Second, 3 * time.Second},
	}
}

func (s *Sender) Publish(ctx context.Context, evt Envelope) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal webhook event: %w", err)
	}

	var lastErr error

	for attempt := 1; attempt <= s.Attempts; attempt++ {
		retryable, err := s.sendOnce(ctx, evt, body)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == s.Attempts {
			break
		}
		if !retryable {
			break
		}

		backoff := s.backoffForAttempt(attempt)
		if err := sleepWithContext(ctx, backoff); err != nil {
			return fmt.Errorf("webhook delivery aborted during backoff: %w", err)
		}
	}

	return lastErr
}

func (s *Sender) sendOnce(ctx context.Context, evt Envelope, body []byte) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("build webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(EventHeader, string(evt.Type))
	req.Header.Set(DeliveryHeader, evt.ID)
	req.Header.Set(SignatureHeader, Sign(s.Secret, body))

	resp, err := s.Client.Do(req)
	if err != nil {
		reason := retryReason(nil, err)
		return shouldRetryReason(reason), fmt.Errorf("send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return false, nil
	}

	reason := retryReason(resp, nil)

	if shouldRetryReason(reason) {
		return true, fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
	}

	if shouldNotRetryReason(reason) {
		return false, fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
	}

	return false, fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
}

func (s *Sender) backoffForAttempt(attempt int) time.Duration {
	if len(s.Backoff) == 0 {
		return 0
	}

	index := attempt - 1
	if index < len(s.Backoff) {
		return s.Backoff[index]
	}

	return s.Backoff[len(s.Backoff)-1]
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryReason(resp *http.Response, err error) string {
	if err != nil {
		return RetryReasonNetworkError
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return RetryReasonHTTP429
	}

	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return RetryReasonHTTP5xx
	}

	if resp.StatusCode >= 400 && resp.StatusCode <= 499 {
		return RetryReasonOtherHTTP4xx
	}

	return ""
}

func shouldRetryReason(reason string) bool {
	return slices.Contains(RetryOn, reason)
}

func shouldNotRetryReason(reason string) bool {
	return slices.Contains(RetryNotOn, reason)
}
