package admin

import (
	"context"
	"encoding/json"
	"log/slog"
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

	jobs, err := s.store.ListRecentFailedEmailJobs(ctx, page.Size, offset)
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
		Limit:  page.Size,
		Offset: (page.Page - 1) * page.Size,
	})
	if err != nil {
		return AuditEventPage{}, err
	}
	return AuditEventPage{Events: events, Page: page.Page, Size: page.Size}, nil
}

func (s *Service) CleanupExpiredAuditEvents(ctx context.Context, now time.Time) (int64, error) {
	if s.auditRetention <= 0 {
		return 0, nil
	}
	return s.store.DeleteAdminAuditEventsBefore(ctx, now.Add(-s.auditRetention))
}

func (s *Service) StartAuditCleanupWorker(ctx context.Context, logger *slog.Logger, interval time.Duration) {
	if s.auditRetention <= 0 || interval <= 0 {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		logger.Info("starting admin audit cleanup worker", "interval", interval.String(), "retention", s.auditRetention.String())
		for {
			select {
			case <-ctx.Done():
				logger.Info("stopping admin audit cleanup worker")
				return
			case now := <-ticker.C:
				cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				deleted, err := s.CleanupExpiredAuditEvents(cleanupCtx, now.UTC())
				cancel()
				if err != nil {
					logger.Error("admin audit cleanup failed", "err", err)
					continue
				}
				if deleted > 0 {
					logger.Info("admin audit events cleaned up", "deleted", deleted)
				}
			}
		}
	}()
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
