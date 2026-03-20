package webhook

import (
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventUserCreated EventType = "user.created"
	EventUserDeleted EventType = "user.deleted"
)

var SupportedEventTypes = []EventType{
	EventUserCreated,
	EventUserDeleted,
}

type Envelope struct {
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Data      any       `json:"data"`
}

type UserData struct {
	UserID uuid.UUID `json:"user_id"`
}

func NewUserCreated(userID uuid.UUID, now time.Time) Envelope {
	return Envelope{
		ID:        uuid.NewString(),
		Type:      EventUserCreated,
		CreatedAt: now.UTC(),
		Data: UserData{
			UserID: userID,
		},
	}
}

func NewUserDeleted(userID uuid.UUID, now time.Time) Envelope {
	return Envelope{
		ID:        uuid.NewString(),
		Type:      EventUserDeleted,
		CreatedAt: now.UTC(),
		Data: UserData{
			UserID: userID,
		},
	}
}
