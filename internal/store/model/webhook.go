package model

import "time"

type WebhookEvent struct {
	ID        string    `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	EventType string `db:"event_type"`
	Payload   []byte `db:"payload"`
	Status    string `db:"status"`

	AttemptCount        int        `db:"attempt_count"`
	NextAttemptAt       time.Time  `db:"next_attempt_at"`
	ProcessingStartedAt *time.Time `db:"processing_started_at"`
	LastError           *string    `db:"last_error"`
	DeliveredAt         *time.Time `db:"delivered_at"`
}

func (WebhookEvent) TableName() string {
	return "webhook_events"
}
