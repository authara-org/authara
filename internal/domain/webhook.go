package domain

import "time"

type WebhookEventStatus string

const (
	WebhookEventStatusPending    WebhookEventStatus = "pending"
	WebhookEventStatusProcessing WebhookEventStatus = "processing"
	WebhookEventStatusDelivered  WebhookEventStatus = "delivered"
	WebhookEventStatusFailed     WebhookEventStatus = "failed"
)

type WebhookEvent struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time

	EventType string
	Payload   []byte
	Status    WebhookEventStatus

	AttemptCount        int
	ProcessingStartedAt *time.Time
	LastError           *string
	DeliveredAt         *time.Time
}
