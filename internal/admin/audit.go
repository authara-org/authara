package admin

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

const (
	ActionUserDisabled          = "user.disabled"
	ActionUserEnabled           = "user.enabled"
	ActionUserAdminGranted      = "user.admin_granted"
	ActionUserAdminRevoked      = "user.admin_revoked"
	ActionUserSessionRevoked    = "user.session_revoked"
	ActionUserSessionsRevoked   = "user.sessions_revoked"
	ActionAllowlistEmailAdded   = "allowlist.email_added"
	ActionAllowlistEmailRemoved = "allowlist.email_removed"
)

func (s *Service) RecentFailures(ctx context.Context, page Page) (RecentFailures, error) {
	page = normalizePage(page, 25)
	offset := (page.Page - 1) * page.Size

	jobs, err := s.store.ListActiveOrFailedEmailJobs(ctx, page.Size, offset)
	if err != nil {
		return RecentFailures{}, err
	}
	challenges, err := s.store.ListRecentRiskyChallenges(ctx, s.now(), page.Size, offset)
	if err != nil {
		return RecentFailures{}, err
	}
	return RecentFailures{
		EmailJobs:  jobs,
		Challenges: challenges,
		Page:       page.Page,
		Size:       page.Size,
	}, nil
}

func (s *Service) ListAuditEvents(ctx context.Context, page Page) (AuditEventPage, error) {
	page = normalizePage(page, 50)
	events, err := s.store.ListAdminAuditEvents(ctx, store.AdminAuditEventFilter{
		Limit:  page.Size + 1,
		Offset: (page.Page - 1) * page.Size,
	})
	if err != nil {
		return AuditEventPage{}, err
	}
	hasNext := len(events) > page.Size
	if hasNext {
		events = events[:page.Size]
	}
	return AuditEventPage{Events: events, Page: page.Page, Size: page.Size, HasNext: hasNext}, nil
}

func (s *Service) ListSecurityEvents(ctx context.Context, page Page) (SecurityEventPage, error) {
	page = normalizePage(page, 50)
	events, err := s.securityEvents.Query(ctx, store.SecurityEventFilter{Limit: page.Size + 1, Offset: (page.Page - 1) * page.Size})
	if err != nil {
		return SecurityEventPage{}, err
	}
	hasNext := len(events) > page.Size
	if hasNext {
		events = events[:page.Size]
	}
	return SecurityEventPage{Events: events, Page: page.Page, Size: page.Size, HasNext: hasNext}, nil
}

func (s *Service) CleanupExpiredAuditEventsBatch(ctx context.Context, now time.Time, batchSize int) (int64, bool, error) {
	retention := s.policy.CurrentAdmin().AuditRetention
	if retention <= 0 {
		return 0, false, nil
	}
	deleted, err := s.store.DeleteAdminAuditEventsBefore(ctx, now.Add(-retention), batchSize)
	return deleted, deleted == int64(batchSize), err
}

func (s *Service) audit(
	ctx context.Context,
	actor Actor,
	action string,
	targetUserID *uuid.UUID,
	targetEmail string,
	metadata map[string]any,
	meta RequestMeta,
) error {
	raw, err := json.Marshal(sanitizeAuditMetadata(metadata))
	if err != nil {
		return err
	}

	actorID := actor.UserID
	var targetEmailPtr *string
	if targetEmail != "" {
		targetEmailPtr = &targetEmail
	}

	var ip *string
	if meta.IP != "" {
		ip = &meta.IP
	}
	var userAgent *string
	if meta.UserAgent != "" {
		userAgent = &meta.UserAgent
	}

	_, err = s.store.CreateAdminAuditEvent(ctx, domain.AdminAuditEvent{
		ActorUserID:  &actorID,
		Action:       action,
		TargetUserID: targetUserID,
		TargetEmail:  targetEmailPtr,
		Metadata:     raw,
		IP:           ip,
		UserAgent:    userAgent,
	})
	return err
}

func sanitizeAuditMetadata(metadata map[string]any) map[string]any {
	out := make(map[string]any)
	for key, value := range metadata {
		if isSensitiveAuditMetadataKey(key) {
			continue
		}
		out[key] = sanitizeAuditMetadataValue(value)
	}
	return out
}

func sanitizeAuditMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeAuditMetadata(typed)
	case string:
		if len(typed) > 512 {
			return typed[:512]
		}
		return typed
	default:
		return value
	}
}

func isSensitiveAuditMetadataKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, token := range []string{
		"password",
		"token",
		"hash",
		"secret",
		"code",
		"credential",
		"public_key",
		"request_body",
		"body",
	} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}
