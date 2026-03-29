package domain

import "github.com/google/uuid"

type AllowedMail struct {
	ID    uuid.UUID
	Email string
}
