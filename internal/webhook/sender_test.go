package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSender_Publish_ReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()

	sender := NewSender(srv.URL, "secret", srv.Client())
	sender.Backoff = []time.Duration{0, 0}

	err := sender.Publish(context.Background(), NewUserCreated(uuid.New(), time.Now()))
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

func TestSender_Publish_RetriesOnServerError(t *testing.T) {
	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("temporary"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewSender(srv.URL, "secret", srv.Client())
	sender.Backoff = []time.Duration{0, 0}

	err := sender.Publish(context.Background(), NewUserCreated(uuid.New(), time.Now()))
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestSender_Publish_DoesNotRetryOnBadRequest(t *testing.T) {
	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	sender := NewSender(srv.URL, "secret", srv.Client())
	sender.Backoff = []time.Duration{0, 0}

	err := sender.Publish(context.Background(), NewUserCreated(uuid.New(), time.Now()))
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestSender_Publish_SendsExpectedHeadersAndBody(t *testing.T) {
	var gotEvent string
	var gotDelivery string
	var gotSignature string
	var gotEnvelope Envelope

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEvent = r.Header.Get("X-Authara-Event")
		gotDelivery = r.Header.Get("X-Authara-Delivery")
		gotSignature = r.Header.Get("X-Authara-Signature")

		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %q", ct)
		}

		if err := json.NewDecoder(r.Body).Decode(&gotEnvelope); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewSender(srv.URL, "secret", srv.Client())
	evt := NewUserCreated(uuid.New(), time.Now())

	if err := sender.Publish(context.Background(), evt); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if gotEvent != string(EventUserCreated) {
		t.Fatalf("expected X-Authara-Event=%q, got %q", EventUserCreated, gotEvent)
	}
	if gotDelivery != evt.ID {
		t.Fatalf("expected X-Authara-Delivery=%q, got %q", evt.ID, gotDelivery)
	}
	if gotSignature == "" {
		t.Fatal("expected non-empty X-Authara-Signature")
	}
	if gotEnvelope.ID != evt.ID {
		t.Fatalf("expected envelope ID=%q, got %q", evt.ID, gotEnvelope.ID)
	}
	if gotEnvelope.Type != evt.Type {
		t.Fatalf("expected envelope Type=%q, got %q", evt.Type, gotEnvelope.Type)
	}
}
