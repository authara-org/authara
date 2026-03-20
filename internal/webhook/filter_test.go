package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recordingInnerPublisher struct {
	events []Envelope
}

func (p *recordingInnerPublisher) Publish(ctx context.Context, evt Envelope) error {
	p.events = append(p.events, evt)
	return nil
}

func TestFilteringPublisher_PublishesOnlyEnabledEvents(t *testing.T) {
	inner := &recordingInnerPublisher{}
	pub := NewFilteringPublisher(inner, map[string]struct{}{
		"user.created": {},
	})

	if err := pub.Publish(context.Background(), NewUserCreated(uuid.New(), time.Now())); err != nil {
		t.Fatalf("Publish user.created failed: %v", err)
	}
	if err := pub.Publish(context.Background(), NewUserDeleted(uuid.New(), time.Now())); err != nil {
		t.Fatalf("Publish user.deleted failed: %v", err)
	}

	if len(inner.events) != 1 {
		t.Fatalf("expected 1 forwarded event, got %d", len(inner.events))
	}
	if inner.events[0].Type != EventUserCreated {
		t.Fatalf("expected forwarded event %q, got %q", EventUserCreated, inner.events[0].Type)
	}
}

func TestFilteringPublisher_PublishesAllWhenEnabledSetIsEmpty(t *testing.T) {
	inner := &recordingInnerPublisher{}
	pub := NewFilteringPublisher(inner, map[string]struct{}{})

	if err := pub.Publish(context.Background(), NewUserCreated(uuid.New(), time.Now())); err != nil {
		t.Fatalf("Publish user.created failed: %v", err)
	}
	if err := pub.Publish(context.Background(), NewUserDeleted(uuid.New(), time.Now())); err != nil {
		t.Fatalf("Publish user.deleted failed: %v", err)
	}

	if len(inner.events) != 2 {
		t.Fatalf("expected 2 forwarded events, got %d", len(inner.events))
	}

	if inner.events[0].Type != EventUserCreated {
		t.Fatalf("expected first event %q, got %q", EventUserCreated, inner.events[0].Type)
	}
	if inner.events[1].Type != EventUserDeleted {
		t.Fatalf("expected second event %q, got %q", EventUserDeleted, inner.events[1].Type)
	}
}
