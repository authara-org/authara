package securityevent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

type Authentication struct {
	Outcome              domain.SecurityEventOutcome
	ReasonCode           string
	ActorType            domain.SecurityEventActorType
	ActorUserID          *uuid.UUID
	UserID               *uuid.UUID
	SessionID            *uuid.UUID
	OrganizationID       *uuid.UUID
	PasskeyID            *uuid.UUID
	AuthenticationMethod domain.AuthenticationMethod
}

type Session struct {
	Outcome              domain.SecurityEventOutcome
	ReasonCode           string
	ActorType            domain.SecurityEventActorType
	ActorUserID          *uuid.UUID
	UserID               *uuid.UUID
	SessionID            *uuid.UUID
	OrganizationID       *uuid.UUID
	AuthenticationMethod domain.AuthenticationMethod
}

type Credential struct {
	ActorType   domain.SecurityEventActorType
	ActorUserID *uuid.UUID
	UserID      uuid.UUID
	SessionID   *uuid.UUID
	PasskeyID   *uuid.UUID
	Provider    domain.Provider
}

type PasskeyClone struct {
	UserID    uuid.UUID
	PasskeyID uuid.UUID
	Outcome   domain.SecurityEventOutcome
	Response  string
}

type Recorder interface {
	AuthenticationLogin(context.Context, Authentication) error
	AuthenticationReauthenticated(context.Context, Authentication) error
	SessionRefresh(context.Context, Session) error
	SessionRefreshTokenReuse(context.Context, Session) error
	SessionLogout(context.Context, Session) error
	SessionRevoked(context.Context, Session) error
	CredentialPasswordAdded(context.Context, Credential) error
	CredentialPasswordChanged(context.Context, Credential) error
	CredentialPasswordReset(context.Context, Credential) error
	CredentialProviderLinked(context.Context, Credential) error
	CredentialProviderChanged(context.Context, Credential) error
	CredentialProviderRemoved(context.Context, Credential) error
	CredentialPasskeyAdded(context.Context, Credential) error
	CredentialPasskeyRemoved(context.Context, Credential) error
	AccountEmailChanged(context.Context, Credential) error
	PasskeyCloneWarning(context.Context, PasskeyClone) error
}

type Config struct {
	Store         *store.Store
	EnabledEvents map[domain.SecurityEventType]struct{}
	Retention     time.Duration
}

type Service struct {
	store         *store.Store
	enabledEvents map[domain.SecurityEventType]struct{}
	retention     time.Duration
}

var ErrStoreRequired = errors.New("security event store is required")

func New(cfg Config) *Service {
	enabled := make(map[domain.SecurityEventType]struct{}, len(cfg.EnabledEvents))
	for eventType := range cfg.EnabledEvents {
		enabled[eventType] = struct{}{}
	}
	return &Service{store: cfg.Store, enabledEvents: enabled, retention: cfg.Retention}
}

func NewStandard(st *store.Store, retention time.Duration) *Service {
	enabled := make(map[domain.SecurityEventType]struct{})
	for _, eventType := range domain.StandardSecurityEventTypes() {
		enabled[eventType] = struct{}{}
	}
	return New(Config{Store: st, EnabledEvents: enabled, Retention: retention})
}

func (s *Service) Enabled(eventType domain.SecurityEventType) bool {
	if s == nil {
		return false
	}
	_, enabled := s.enabledEvents[eventType]
	return enabled
}

func (s *Service) AuthenticationLogin(ctx context.Context, event Authentication) error {
	return s.recordAuthentication(ctx, domain.SecurityEventAuthenticationLogin, event)
}

func (s *Service) AuthenticationReauthenticated(ctx context.Context, event Authentication) error {
	return s.recordAuthentication(ctx, domain.SecurityEventAuthenticationReauthenticated, event)
}

func (s *Service) recordAuthentication(ctx context.Context, eventType domain.SecurityEventType, event Authentication) error {
	return s.record(ctx, domain.SecurityEvent{
		Type: eventType, Outcome: event.Outcome, ReasonCode: event.ReasonCode,
		ActorType: event.ActorType, ActorUserID: event.ActorUserID, UserID: event.UserID,
		SessionID: event.SessionID, OrganizationID: event.OrganizationID, PasskeyID: event.PasskeyID,
		AuthenticationMethod: event.AuthenticationMethod,
	})
}

func (s *Service) SessionRefresh(ctx context.Context, event Session) error {
	return s.recordSession(ctx, domain.SecurityEventSessionRefresh, event)
}

func (s *Service) SessionRefreshTokenReuse(ctx context.Context, event Session) error {
	return s.recordSession(ctx, domain.SecurityEventSessionRefreshTokenReuse, event)
}

func (s *Service) SessionLogout(ctx context.Context, event Session) error {
	return s.recordSession(ctx, domain.SecurityEventSessionLogout, event)
}

func (s *Service) SessionRevoked(ctx context.Context, event Session) error {
	return s.recordSession(ctx, domain.SecurityEventSessionRevoked, event)
}

func (s *Service) recordSession(ctx context.Context, eventType domain.SecurityEventType, event Session) error {
	return s.record(ctx, domain.SecurityEvent{
		Type: eventType, Outcome: event.Outcome, ReasonCode: event.ReasonCode,
		ActorType: event.ActorType, ActorUserID: event.ActorUserID, UserID: event.UserID,
		SessionID: event.SessionID, OrganizationID: event.OrganizationID,
		AuthenticationMethod: event.AuthenticationMethod,
	})
}

func (s *Service) CredentialPasswordAdded(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventCredentialPasswordAdded, event)
}

func (s *Service) CredentialPasswordChanged(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventCredentialPasswordChanged, event)
}

func (s *Service) CredentialPasswordReset(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventCredentialPasswordReset, event)
}

func (s *Service) CredentialProviderLinked(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventCredentialProviderLinked, event)
}

func (s *Service) CredentialProviderChanged(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventCredentialProviderChanged, event)
}

func (s *Service) CredentialProviderRemoved(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventCredentialProviderRemoved, event)
}

func (s *Service) CredentialPasskeyAdded(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventCredentialPasskeyAdded, event)
}

func (s *Service) CredentialPasskeyRemoved(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventCredentialPasskeyRemoved, event)
}

func (s *Service) AccountEmailChanged(ctx context.Context, event Credential) error {
	return s.recordCredential(ctx, domain.SecurityEventAccountEmailChanged, event)
}

func (s *Service) recordCredential(ctx context.Context, eventType domain.SecurityEventType, event Credential) error {
	actorType := event.ActorType
	if actorType == "" {
		actorType = domain.SecurityEventActorUser
	}
	actorUserID := event.ActorUserID
	if actorUserID == nil && actorType == domain.SecurityEventActorUser {
		actorUserID = &event.UserID
	}
	return s.record(ctx, domain.SecurityEvent{
		Type: eventType, Outcome: domain.SecurityEventOutcomeSuccess,
		ActorType: actorType, ActorUserID: actorUserID, UserID: &event.UserID,
		SessionID: event.SessionID, PasskeyID: event.PasskeyID, Response: string(event.Provider),
	})
}

func (s *Service) PasskeyCloneWarning(ctx context.Context, event PasskeyClone) error {
	return s.record(ctx, domain.SecurityEvent{
		Type: domain.SecurityEventPasskeyCloneWarning, Outcome: event.Outcome,
		ActorType: domain.SecurityEventActorSystem, UserID: &event.UserID,
		PasskeyID: &event.PasskeyID, Response: event.Response,
	})
}

func (s *Service) record(ctx context.Context, event domain.SecurityEvent) error {
	if !s.Enabled(event.Type) {
		return nil
	}
	if s.store == nil {
		return ErrStoreRequired
	}
	_, err := s.store.CreateSecurityEvent(ctx, event)
	return err
}

type Filter = store.SecurityEventFilter

func (s *Service) Query(ctx context.Context, filter Filter) ([]domain.SecurityEvent, error) {
	return s.store.QuerySecurityEvents(ctx, filter)
}

func (s *Service) ExportNDJSON(ctx context.Context, filter Filter, dst io.Writer) error {
	encoder := json.NewEncoder(dst)
	filter.Limit = 200
	filter.Offset = 0
	for {
		events, err := s.store.QuerySecurityEvents(ctx, filter)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err := encoder.Encode(event); err != nil {
				return err
			}
		}
		if len(events) < filter.Limit {
			return nil
		}
		last := events[len(events)-1]
		filter.CursorCreatedAt = &last.CreatedAt
		filter.CursorID = &last.ID
	}
}

func (s *Service) CleanupExpired(ctx context.Context, now time.Time) (int64, error) {
	if s == nil || s.retention <= 0 {
		return 0, nil
	}
	return s.store.DeleteSecurityEventsBefore(ctx, now.Add(-s.retention))
}

func (s *Service) StartCleanupWorker(ctx context.Context, logger *slog.Logger, interval time.Duration) {
	if s == nil || interval <= 0 || s.retention <= 0 {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		logger.Info("starting security-event cleanup worker", "interval", interval.String())
		for {
			select {
			case <-ctx.Done():
				logger.Info("stopping security-event cleanup worker")
				return
			case now := <-ticker.C:
				cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				deleted, err := s.CleanupExpired(cleanupCtx, now.UTC())
				cancel()
				if err != nil {
					logger.Error("security-event cleanup failed", "err", err)
					continue
				}
				if deleted > 0 {
					logger.Info("security events cleaned up", "deleted", deleted)
				}
			}
		}
	}()
}
