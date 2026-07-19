package webhook

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
)

type QueuePublisher struct {
	store *store.Store
}

func NewQueuePublisher(store *store.Store) *QueuePublisher {
	return &QueuePublisher{store: store}
}

func (p *QueuePublisher) Publish(ctx context.Context, event Envelope) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal webhook event: %w", err)
	}

	_, err = p.store.CreateWebhookEvent(ctx, domain.WebhookEvent{
		ID:        event.ID,
		EventType: string(event.Type),
		Payload:   payload,
	})
	if err != nil {
		return fmt.Errorf("enqueue webhook event: %w", err)
	}
	return nil
}
