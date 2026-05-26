package admin

import (
	"context"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/useragent"
)

func (s *Service) userSummary(ctx context.Context, user domain.User) (UserSummary, error) {
	now := s.now()
	roleNames, err := s.store.GetUserPlatformRoleNames(ctx, user.ID)
	if err != nil {
		return UserSummary{}, err
	}
	providers, err := s.store.ListAuthProvidersByUserID(ctx, user.ID)
	if err != nil {
		return UserSummary{}, err
	}
	activeSessions, err := s.store.CountActiveSessionsByUserID(ctx, user.ID, now)
	if err != nil {
		return UserSummary{}, err
	}
	return UserSummary{
		ID:                 user.ID,
		CreatedAt:          user.CreatedAt,
		UpdatedAt:          user.UpdatedAt,
		DisabledAt:         user.DisabledAt,
		Username:           user.Username,
		Email:              user.Email,
		Roles:              roleNames,
		AuthProviderCount:  len(providers),
		ActiveSessionCount: activeSessions,
	}, nil
}

func summarizeAuthProviders(providers []domain.AuthProvider) []AuthProviderSummary {
	out := make([]AuthProviderSummary, 0, len(providers))
	for _, provider := range providers {
		out = append(out, AuthProviderSummary{
			ID:          provider.ID,
			Provider:    string(provider.Provider),
			CreatedAt:   provider.CreatedAt,
			HasPassword: provider.PasswordHash != nil,
			HasOAuthID:  provider.ProviderUserID != nil,
		})
	}
	return out
}

func summarizePasskeys(passkeys []domain.Passkey) []PasskeySummary {
	out := make([]PasskeySummary, 0, len(passkeys))
	for _, passkey := range passkeys {
		out = append(out, PasskeySummary{
			ID:             passkey.ID,
			Name:           passkey.Name,
			CreatedAt:      passkey.CreatedAt,
			LastUsedAt:     passkey.LastUsedAt,
			CloneWarning:   passkey.CloneWarning,
			BackupEligible: passkey.BackupEligible,
			BackupState:    passkey.BackupState,
			DeviceLabel:    passkeyDeviceLabel(passkey.Transport),
			Transport:      passkey.Transport,
		})
	}
	return out
}

func summarizeSessions(sessions []domain.Session, now time.Time) []SessionSummary {
	out := make([]SessionSummary, 0, len(sessions))
	for _, session := range sessions {
		status := "Active"
		switch {
		case session.RevokedAt != nil:
			status = "Revoked"
		case !session.ExpiresAt.After(now):
			status = "Expired"
		}
		out = append(out, SessionSummary{
			ID:               session.ID,
			CreatedAt:        session.CreatedAt,
			ExpiresAt:        session.ExpiresAt,
			RevokedAt:        session.RevokedAt,
			UserAgent:        session.UserAgent,
			UserAgentSummary: useragent.Parse(session.UserAgent).BrowserSummary(),
			Status:           status,
		})
	}
	return out
}

func passkeyDeviceLabel(transports []string) string {
	if len(transports) == 0 {
		return "Passkey"
	}
	for _, transport := range transports {
		switch strings.ToLower(strings.TrimSpace(transport)) {
		case "internal":
			return "Platform authenticator"
		case "hybrid":
			return "Phone or cross-device passkey"
		case "usb", "nfc", "ble":
			return "Security key"
		}
	}
	return "Passkey"
}
