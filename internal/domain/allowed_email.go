package domain

import "github.com/google/uuid"

type AllowedEmail struct {
	ID    uuid.UUID
	Email string
}
