package admin

import (
	"time"

	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/authara-org/authara/internal/webhook"
)

type Config struct {
	Store            *store.Store
	Tx               *tx.Manager
	Now              func() time.Time
	AllowlistEnabled bool
	AuditRetention   time.Duration
	WebhookPublisher webhook.Publisher
}

type Service struct {
	store            *store.Store
	tx               *tx.Manager
	now              func() time.Time
	allowlistEnabled bool
	auditRetention   time.Duration
	webhookPublisher webhook.Publisher
}

func New(cfg Config) *Service {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	pub := cfg.WebhookPublisher
	if pub == nil {
		pub = webhook.NoopPublisher{}
	}
	return &Service{
		store:            cfg.Store,
		tx:               cfg.Tx,
		now:              now,
		allowlistEnabled: cfg.AllowlistEnabled,
		auditRetention:   cfg.AuditRetention,
		webhookPublisher: pub,
	}
}
