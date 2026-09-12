package admin

import (
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/authara-org/authara/internal/webhook"
)

type Config struct {
	Store                  *store.Store
	Tx                     *tx.Manager
	Now                    func() time.Time
	AllowlistEnabled       bool
	AuditRetention         time.Duration
	Policy                 config.AdminPolicyReader
	AllowlistPolicy        config.AllowlistPolicyReader
	WebhookPublisher       webhook.Publisher
	AccessTokenRevocations *token.AccessTokenRevocations
}

type Service struct {
	store                  *store.Store
	tx                     *tx.Manager
	now                    func() time.Time
	policy                 config.AdminPolicyReader
	allowlistPolicy        config.AllowlistPolicyReader
	webhookPublisher       webhook.Publisher
	accessTokenRevocations *token.AccessTokenRevocations
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
	policy := cfg.Policy
	if policy == nil {
		policy = config.AdminPolicyReaderFunc(func() config.AdminPolicy {
			return config.AdminPolicy{AuditRetention: cfg.AuditRetention}
		})
	}
	allowlistPolicy := cfg.AllowlistPolicy
	if allowlistPolicy == nil {
		allowlistPolicy = config.AllowlistPolicyReaderFunc(func() config.AllowlistPolicy {
			return config.AllowlistPolicy{AllowlistEnabled: cfg.AllowlistEnabled}
		})
	}
	return &Service{
		store:                  cfg.Store,
		tx:                     cfg.Tx,
		now:                    now,
		policy:                 policy,
		allowlistPolicy:        allowlistPolicy,
		webhookPublisher:       pub,
		accessTokenRevocations: cfg.AccessTokenRevocations,
	}
}
