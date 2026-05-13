package viewmodel

import (
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type Passkey struct {
	ID         uuid.UUID
	Name       string
	CreatedAt  string
	LastUsedAt string
	CanDelete  bool
}

func PasskeysFromDomain(passkeys []domain.Passkey, totalAuthMethods int) []Passkey {
	out := make([]Passkey, 0, len(passkeys))
	for _, p := range passkeys {
		out = append(out, Passkey{
			ID:         p.ID,
			Name:       passkeyName(p.Name),
			CreatedAt:  formatAccountTimestamp(p.CreatedAt),
			LastUsedAt: formatOptionalAccountTimestamp(p.LastUsedAt),
			CanDelete:  totalAuthMethods > 1,
		})
	}
	return out
}

func passkeyName(name string) string {
	if name == "" {
		return "Passkey"
	}
	return name
}

func formatOptionalAccountTimestamp(t *time.Time) string {
	if t == nil {
		return "Never used"
	}
	return formatAccountTimestamp(*t)
}

func formatAccountTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("Jan 2, 2006")
}
