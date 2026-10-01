package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/securityevent"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/google/uuid"
)

type SessionConfig struct {
	Store                      *store.Store
	Tx                         *tx.Manager
	AccessTokens               *token.AccessTokenService
	AccessTokenRevocations     *token.AccessTokenRevocations
	SessionTTL                 time.Duration
	RefreshTokenTTL            time.Duration
	RefreshTokenRotation       time.Duration
	RecentAuthenticationWindow time.Duration
	Policy                     config.SessionPolicyReader
	AccessPolicy               accesspolicy.EmailAccessPolicy
	Organizations              *organization.Service
	SecurityEvents             securityevent.Recorder
}

type Service struct {
	store                  *store.Store
	tx                     *tx.Manager
	accessTokens           *token.AccessTokenService
	accessTokenRevocations *token.AccessTokenRevocations
	policy                 config.SessionPolicyReader
	accessPolicy           accesspolicy.EmailAccessPolicy
	organizations          *organization.Service
	securityEvents         securityevent.Recorder
}

const authenticationChallengeTTL = 5 * time.Minute

var errRefreshTokenReuseDetected = errors.New("refresh token reuse detected")

func New(cfg SessionConfig) *Service {
	access := cfg.AccessPolicy
	if access == nil {
		access = accesspolicy.NoopEmailAccessPolicy{}
	}
	policy := cfg.Policy
	if policy == nil {
		recentAuthenticationWindow := cfg.RecentAuthenticationWindow
		if recentAuthenticationWindow <= 0 {
			recentAuthenticationWindow = 10 * time.Minute
		}
		policy = config.SessionPolicyReaderFunc(func() config.SessionPolicy {
			return config.SessionPolicy{
				SessionTTL: cfg.SessionTTL, RefreshTokenTTL: cfg.RefreshTokenTTL,
				RefreshTokenRotation:        cfg.RefreshTokenRotation,
				RecentAuthenticationEnabled: true,
				RecentAuthenticationWindow:  recentAuthenticationWindow,
			}
		})
	}
	securityEvents := cfg.SecurityEvents
	if securityEvents == nil {
		securityEvents = securityevent.NoopRecorder{}
	}

	return &Service{
		store:                  cfg.Store,
		tx:                     cfg.Tx,
		accessTokens:           cfg.AccessTokens,
		accessTokenRevocations: cfg.AccessTokenRevocations,
		policy:                 policy,
		accessPolicy:           access,
		organizations:          cfg.Organizations,
		securityEvents:         securityEvents,
	}
}

func (s *Service) CreateSession(
	ctx context.Context,
	userID uuid.UUID,
	audience token.Audience,
	authenticationMethod domain.AuthenticationMethod,
	userAgent string,
	now time.Time,
	clientIP string,
) (
	accessToken string,
	refreshToken string,
	err error,
) {
	return s.createSession(ctx, userID, nil, audience, authenticationMethod, userAgent, now, clientIP)
}

func (s *Service) CreatePasskeySession(
	ctx context.Context,
	userID uuid.UUID,
	passkeyID uuid.UUID,
	audience token.Audience,
	userAgent string,
	now time.Time,
	clientIP string,
) (accessToken string, refreshToken string, err error) {
	return s.createSession(ctx, userID, &passkeyID, audience, domain.AuthenticationMethodPasskey, userAgent, now, clientIP)
}

func (s *Service) createSession(
	ctx context.Context,
	userID uuid.UUID,
	passkeyID *uuid.UUID,
	audience token.Audience,
	authenticationMethod domain.AuthenticationMethod,
	userAgent string,
	now time.Time,
	clientIP string,
) (
	accessToken string,
	refreshToken string,
	err error,
) {
	policy := s.policy.CurrentSession()
	var resultErr error
	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		user, err := s.ensureUserAllowed(ctx, userID)
		if err != nil {
			if errors.Is(err, ErrUserNotAllowed) {
				resultErr = err
				return s.recordSessionAuthenticationDenied(ctx, userID, authenticationMethod, domain.SecurityEventReasonAccessPolicy)
			}
			return err
		}
		if err := s.store.LockUserForKeyShare(ctx, userID); err != nil {
			return err
		}
		if passkeyID != nil {
			passkey, err := s.store.GetPasskeyByID(ctx, *passkeyID)
			if errors.Is(err, store.ErrPasskeyNotFound) {
				resultErr = ErrAuthenticationMethodUnavailable
				return s.recordSessionAuthenticationDenied(ctx, userID, authenticationMethod, domain.SecurityEventReasonMethodUnavailable)
			}
			if err != nil {
				return err
			}
			if passkey.UserID != userID || passkey.RestrictedAt != nil {
				resultErr = ErrAuthenticationMethodUnavailable
				return s.recordSessionAuthenticationDenied(ctx, userID, authenticationMethod, domain.SecurityEventReasonMethodUnavailable)
			}
		}

		org, membership, err := s.organizations.DefaultOrganizationForUser(ctx, user.ID)
		if err != nil {
			return err
		}
		if err := s.store.LockOrganizationForKeyShare(ctx, org.ID); err != nil {
			return err
		}
		membership, err = s.organizations.RequireMembership(ctx, userID, org.ID)
		if err != nil {
			return err
		}

		roleNames, err := s.store.GetUserPlatformRoleNames(ctx, userID)
		if err != nil {
			return err
		}

		platformRoles, err := roles.FromDBRoleNames(roleNames)
		if err != nil {
			return err
		}

		if !canAccessAudience(platformRoles, audience) {
			resultErr = ErrForbidden
			return s.recordSessionAuthenticationDenied(ctx, userID, authenticationMethod, domain.SecurityEventReasonAudienceForbidden)
		}

		disabled, err := s.store.IsUserDisabled(ctx, userID)
		if err != nil {
			return err
		}
		if disabled {
			resultErr = ErrUserDisabled
			return s.recordSessionAuthenticationDenied(ctx, userID, authenticationMethod, domain.SecurityEventReasonUserDisabled)
		}

		session := domain.Session{
			UserID:               userID,
			ActiveOrganizationID: org.ID,
			UserAgent:            userAgent,
			ExpiresAt:            now.Add(policy.SessionTTL),
			AuthenticatedAt:      &now,
			AuthenticationMethod: authenticationMethod,
		}

		createdSession, err := s.store.CreateSession(ctx, session)
		if err != nil {
			return err
		}

		refreshToken, err = generateRefreshToken()
		if err != nil {
			return err
		}

		hashedRefreshToken := hashRefreshToken(refreshToken)

		rt := domain.RefreshToken{
			SessionID:      createdSession.ID,
			OrganizationID: org.ID,
			TokenHash:      hashedRefreshToken,
			ExpiresAt:      now.Add(policy.RefreshTokenTTL),
		}

		err = s.store.CreateRefreshToken(ctx, rt)
		if err != nil {
			return err
		}

		accessToken, err = s.accessTokens.GenerateWithEmailVerification(
			userID,
			createdSession.ID,
			org.ID,
			string(membership.Role),
			audience,
			platformRoles,
			user.EmailVerifiedAt != nil,
			now,
		)
		if err != nil {
			return err
		}
		if err := email.Enqueue(ctx, s.store, user.Email, domain.EmailTemplateNewSignIn, email.TemplateData{
			email.TemplateVariableIPAddress:  email.ValueOrUnknown(clientIP),
			email.TemplateVariableUserAgent:  email.ValueOrUnknown(userAgent),
			email.TemplateVariableOccurredAt: email.OccurredAt(now),
		}, now); err != nil {
			return err
		}
		if err := s.securityEvents.AuthenticationLogin(ctx, securityevent.Authentication{
			Outcome:              domain.SecurityEventOutcomeSuccess,
			ActorType:            domain.SecurityEventActorUser,
			ActorUserID:          &userID,
			UserID:               &userID,
			SessionID:            &createdSession.ID,
			OrganizationID:       &org.ID,
			PasskeyID:            passkeyID,
			AuthenticationMethod: authenticationMethod,
		}); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", "", err
	}
	if resultErr != nil {
		return "", "", resultErr
	}

	return accessToken, refreshToken, nil
}

func (s *Service) recordSessionAuthenticationDenied(
	ctx context.Context,
	userID uuid.UUID,
	method domain.AuthenticationMethod,
	reason string,
) error {
	return s.securityEvents.AuthenticationLogin(ctx, securityevent.Authentication{
		Outcome:              domain.SecurityEventOutcomeDenied,
		ReasonCode:           reason,
		ActorType:            domain.SecurityEventActorUser,
		ActorUserID:          &userID,
		UserID:               &userID,
		AuthenticationMethod: method,
	})
}

func (s *Service) RequireRecentAuthentication(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	now time.Time,
) error {
	policy := s.policy.CurrentSession()
	current, err := s.store.GetActiveSessionByID(ctx, sessionID, now)
	if err != nil {
		if errors.Is(err, store.ErrSessionNotFound) {
			return ErrInvalidSession
		}
		return err
	}
	if current.UserID != userID {
		return ErrInvalidSession
	}
	if !policy.RecentAuthenticationEnabled {
		return nil
	}
	window := policy.RecentAuthenticationWindow
	if window <= 0 || current.AuthenticatedAt == nil || current.AuthenticationMethod == "" {
		return ErrRecentAuthenticationRequired
	}
	if current.AuthenticatedAt.After(now) || now.Sub(*current.AuthenticatedAt) > window {
		return ErrRecentAuthenticationRequired
	}
	return nil
}

func (s *Service) MarkRecentlyAuthenticated(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	method domain.AuthenticationMethod,
	now time.Time,
) error {
	switch method {
	case domain.AuthenticationMethodPassword, domain.AuthenticationMethodPasskey, domain.AuthenticationMethodGoogle:
	default:
		return ErrRecentAuthenticationRequired
	}
	return s.store.UpdateSessionAuthentication(ctx, userID, sessionID, method, now)
}

func (s *Service) StartAuthenticationChallenge(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	now time.Time,
) (domain.AuthenticationChallenge, error) {
	var challenge domain.AuthenticationChallenge
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.store.GetActiveSessionByIDForUpdate(txCtx, sessionID, now)
		if err != nil {
			if errors.Is(err, store.ErrSessionNotFound) {
				return ErrInvalidSession
			}
			return err
		}
		if current.UserID != userID {
			return ErrInvalidSession
		}
		existing, err := s.store.GetAuthenticationChallengeBySessionIDForUpdate(txCtx, sessionID)
		if err == nil && !existing.IsConsumed() && !existing.IsExpired(now) {
			challenge = existing
			return nil
		}
		if err != nil && !errors.Is(err, store.ErrAuthenticationChallengeNotFound) {
			return err
		}
		expiresAt := now.Add(authenticationChallengeTTL)
		if expiresAt.After(current.ExpiresAt) {
			expiresAt = current.ExpiresAt
		}
		challenge, err = s.store.ReplaceAuthenticationChallenge(txCtx, domain.AuthenticationChallenge{
			ID:        uuid.New(),
			UserID:    userID,
			SessionID: sessionID,
			CreatedAt: now,
			ExpiresAt: expiresAt,
		})
		return err
	})
	return challenge, err
}

func (s *Service) ValidateAuthenticationChallenge(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	challengeID uuid.UUID,
	now time.Time,
) error {
	challenge, err := s.store.GetAuthenticationChallengeByID(ctx, challengeID)
	if err != nil {
		if errors.Is(err, store.ErrAuthenticationChallengeNotFound) {
			return ErrAuthenticationChallengeInvalid
		}
		return err
	}
	if challenge.UserID != userID || challenge.SessionID != sessionID || challenge.IsConsumed() || challenge.IsExpired(now) {
		return ErrAuthenticationChallengeInvalid
	}
	return nil
}

func (s *Service) ValidateCompletedAuthenticationChallenge(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	challengeID uuid.UUID,
) error {
	challenge, err := s.store.GetAuthenticationChallengeByID(ctx, challengeID)
	if err != nil {
		if errors.Is(err, store.ErrAuthenticationChallengeNotFound) {
			return ErrAuthenticationChallengeInvalid
		}
		return err
	}
	if challenge.UserID != userID || challenge.SessionID != sessionID || !challenge.IsConsumed() || challenge.AuthenticationMethod == "" {
		return ErrAuthenticationChallengeInvalid
	}
	return nil
}

func (s *Service) CompleteAuthenticationChallenge(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	challengeID uuid.UUID,
	method domain.AuthenticationMethod,
	now time.Time,
) error {
	switch method {
	case domain.AuthenticationMethodPassword, domain.AuthenticationMethodPasskey, domain.AuthenticationMethodGoogle:
	default:
		return ErrAuthenticationChallengeInvalid
	}

	var resultErr error
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.store.GetActiveSessionByIDForUpdate(txCtx, sessionID, now)
		if err != nil {
			if errors.Is(err, store.ErrSessionNotFound) {
				resultErr = ErrAuthenticationChallengeInvalid
				return nil
			}
			return err
		}
		if current.UserID != userID {
			resultErr = ErrAuthenticationChallengeInvalid
			return nil
		}
		challenge, err := s.store.GetAuthenticationChallengeByIDForUpdate(txCtx, challengeID)
		if errors.Is(err, store.ErrAuthenticationChallengeNotFound) {
			resultErr = ErrAuthenticationChallengeInvalid
			return nil
		}
		if err != nil {
			return err
		}
		if challenge.UserID != userID || challenge.SessionID != sessionID || challenge.IsConsumed() || challenge.IsExpired(now) {
			resultErr = ErrAuthenticationChallengeInvalid
			return nil
		}
		if err := s.store.UpdateSessionAuthentication(txCtx, userID, sessionID, method, now); err != nil {
			return err
		}
		if err := s.store.ConsumeAuthenticationChallenge(txCtx, challengeID, method, now); err != nil {
			return err
		}
		return s.securityEvents.AuthenticationReauthenticated(txCtx, securityevent.Authentication{
			Outcome: domain.SecurityEventOutcomeSuccess, ActorType: domain.SecurityEventActorUser,
			ActorUserID: &userID, UserID: &userID, SessionID: &sessionID, AuthenticationMethod: method,
		})
	})
	if err != nil {
		return err
	}
	return resultErr
}

// RecordReauthenticationDenied persists a rejected credential proof after the
// challenge transaction has ended. Keeping this separate from
// CompleteAuthenticationChallenge ensures denial evidence is not rolled back.
func (s *Service) RecordReauthenticationDenied(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	method domain.AuthenticationMethod,
	reasonCode string,
) error {
	return s.securityEvents.AuthenticationReauthenticated(ctx, securityevent.Authentication{
		Outcome:              domain.SecurityEventOutcomeDenied,
		ReasonCode:           reasonCode,
		ActorType:            domain.SecurityEventActorUser,
		ActorUserID:          &userID,
		UserID:               &userID,
		SessionID:            &sessionID,
		AuthenticationMethod: method,
	})
}

func (s *Service) SwitchSessionOrganization(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	organizationID uuid.UUID,
	audience token.Audience,
	now time.Time,
) (
	accessToken string,
	refreshToken string,
	err error,
) {
	policy := s.policy.CurrentSession()
	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		user, err := s.ensureUserAllowed(ctx, userID)
		if err != nil {
			return err
		}
		if err := s.store.LockUserForKeyShare(ctx, userID); err != nil {
			return err
		}
		if err := s.store.LockOrganizationForKeyShare(ctx, organizationID); err != nil {
			if errors.Is(err, store.ErrOrganizationNotFound) {
				return ErrForbidden
			}
			return err
		}

		session, err := s.store.GetActiveSessionByID(ctx, sessionID, now)
		if err != nil {
			return ErrInvalidSession
		}
		if session.UserID != userID {
			return ErrForbidden
		}

		membership, err := s.organizations.RequireMembership(ctx, userID, organizationID)
		if err != nil {
			if errors.Is(err, store.ErrOrganizationMembershipNotFound) {
				return ErrForbidden
			}
			return err
		}

		roleNames, err := s.store.GetUserPlatformRoleNames(ctx, userID)
		if err != nil {
			return err
		}
		platformRoles, err := roles.FromDBRoleNames(roleNames)
		if err != nil {
			return err
		}
		if !canAccessAudience(platformRoles, audience) {
			return ErrForbidden
		}

		disabled, err := s.store.IsUserDisabled(ctx, userID)
		if err != nil {
			return err
		}
		if disabled {
			return ErrUserDisabled
		}

		if err := s.store.UpdateSessionActiveOrganization(ctx, sessionID, organizationID); err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensBySession(ctx, sessionID); err != nil {
			return err
		}

		refreshToken, err = generateRefreshToken()
		if err != nil {
			return err
		}
		expiresAt := now.Add(policy.RefreshTokenTTL)
		if expiresAt.After(session.ExpiresAt) {
			expiresAt = session.ExpiresAt
		}
		if err := s.store.CreateRefreshToken(ctx, domain.RefreshToken{
			SessionID:      sessionID,
			OrganizationID: organizationID,
			TokenHash:      hashRefreshToken(refreshToken),
			CreatedAt:      now,
			ExpiresAt:      expiresAt,
		}); err != nil {
			return err
		}

		accessToken, err = s.accessTokens.GenerateWithEmailVerification(
			userID,
			sessionID,
			organizationID,
			string(membership.Role),
			audience,
			platformRoles,
			user.EmailVerifiedAt != nil,
			now,
		)
		return err
	})
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (s *Service) RefreshSession(ctx context.Context, refreshToken string, audience token.Audience, now time.Time) (newAccessToken string, newRefreshToken string, err error) {
	policy := s.policy.CurrentSession()
	var reusedRefreshToken domain.RefreshToken

	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		hashed := hashRefreshToken(refreshToken)

		rt, err := s.store.GetRefreshTokenByHash(ctx, hashed)
		if err != nil {
			return ErrInvalidRefreshToken
		}
		if rt.ConsumedAt != nil {
			reusedRefreshToken = rt
			return errRefreshTokenReuseDetected
		}
		if rt.ExpiresAt.Before(now) {
			return ErrInvalidRefreshToken
		}

		session, err := s.store.GetSessionByID(ctx, rt.SessionID)
		if err != nil {
			return ErrInvalidRefreshToken
		}
		if err := s.store.LockUserForKeyShare(ctx, session.UserID); err != nil {
			return ErrInvalidRefreshToken
		}
		if err := s.store.LockOrganizationForKeyShare(ctx, rt.OrganizationID); err != nil {
			return ErrInvalidRefreshToken
		}

		rt, err = s.store.GetRefreshTokenByHashForUpdate(ctx, hashed)
		if err != nil {
			return ErrInvalidRefreshToken
		}
		if rt.ConsumedAt != nil {
			reusedRefreshToken = rt
			return errRefreshTokenReuseDetected
		}
		if rt.ExpiresAt.Before(now) {
			return ErrInvalidRefreshToken
		}
		session, err = s.store.GetSessionByID(ctx, rt.SessionID)
		if err != nil {
			return ErrInvalidRefreshToken
		}
		if session.ExpiresAt.Before(now) || session.RevokedAt != nil {
			return ErrInvalidRefreshToken
		}

		user, err := s.ensureUserAllowed(ctx, session.UserID)
		if err != nil {
			return err
		}
		membership, err := s.organizations.RequireMembership(ctx, session.UserID, rt.OrganizationID)
		if err != nil {
			if errors.Is(err, store.ErrOrganizationMembershipNotFound) {
				return ErrForbidden
			}
			return err
		}

		roleNames, err := s.store.GetUserPlatformRoleNames(ctx, session.UserID)
		if err != nil {
			return err
		}

		platformRoles, err := roles.FromDBRoleNames(roleNames)
		if err != nil {
			return err
		}

		if !canAccessAudience(platformRoles, audience) {
			return ErrForbidden
		}

		disabled, err := s.store.IsUserDisabled(ctx, session.UserID)
		if err != nil {
			return err
		}
		if disabled {
			return ErrUserDisabled
		}

		needToRotate := shouldRotate(rt, now, policy.RefreshTokenRotation)
		if needToRotate {
			err = s.store.ConsumeRefreshToken(ctx, rt.ID, now)
			if err != nil {
				if errors.Is(err, store.ErrRefreshTokenNotFound) {
					return ErrInvalidRefreshToken
				}
				return err
			}

			newRefreshToken, err = generateRefreshToken()
			if err != nil {
				return err
			}

			newHashed := hashRefreshToken(newRefreshToken)
			newExpiresAt := now.Add(policy.RefreshTokenTTL)
			if newExpiresAt.After(session.ExpiresAt) {
				newExpiresAt = session.ExpiresAt
			}

			newRT := domain.RefreshToken{
				SessionID:      rt.SessionID,
				OrganizationID: rt.OrganizationID,
				TokenHash:      newHashed,
				CreatedAt:      now,
				ExpiresAt:      newExpiresAt,
			}

			err = s.store.CreateRefreshToken(ctx, newRT)
			if err != nil {
				return err
			}
		} else {
			newRefreshToken = refreshToken
		}

		newAccessToken, err = s.accessTokens.GenerateWithEmailVerification(
			session.UserID,
			rt.SessionID,
			rt.OrganizationID,
			string(membership.Role),
			audience,
			platformRoles,
			user.EmailVerifiedAt != nil,
			now,
		)
		if err != nil {
			return err
		}
		if err := s.securityEvents.SessionRefresh(ctx, securityevent.Session{
			Outcome:              domain.SecurityEventOutcomeSuccess,
			ActorType:            domain.SecurityEventActorUser,
			ActorUserID:          &session.UserID,
			UserID:               &session.UserID,
			SessionID:            &session.ID,
			OrganizationID:       &rt.OrganizationID,
			AuthenticationMethod: session.AuthenticationMethod,
		}); err != nil {
			return err
		}

		return nil
	})

	if errors.Is(err, errRefreshTokenReuseDetected) {
		return "", "", s.revokeReusedRefreshToken(ctx, reusedRefreshToken, now)
	}
	if err != nil {
		return "", "", err
	}

	return newAccessToken, newRefreshToken, nil
}

func (s *Service) revokeReusedRefreshToken(
	ctx context.Context,
	rt domain.RefreshToken,
	observedAt time.Time,
) error {
	var cacheErr error
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		session, err := s.store.GetSessionByID(txCtx, rt.SessionID)
		if err != nil {
			return err
		}
		if err := s.store.LockUserForUpdate(txCtx, session.UserID); err != nil {
			return err
		}
		if _, err := s.store.GetSessionByID(txCtx, rt.SessionID); err != nil {
			return err
		}

		markerAt := time.Now().UTC()
		if observedAt.After(markerAt) {
			markerAt = observedAt
		}
		cacheErr = s.accessTokenRevocations.RevokeSession(txCtx, rt.SessionID, markerAt)
		if err := s.store.RevokeSession(txCtx, rt.SessionID, observedAt); err != nil {
			return errors.Join(cacheErr, err)
		}
		if err := s.securityEvents.SessionRefreshTokenReuse(txCtx, securityevent.Session{
			Outcome:        domain.SecurityEventOutcomeDenied,
			ReasonCode:     domain.SecurityEventReasonRefreshTokenReuse,
			ActorType:      domain.SecurityEventActorAnonymous,
			UserID:         &session.UserID,
			SessionID:      &session.ID,
			OrganizationID: &rt.OrganizationID,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return errors.Join(ErrRefreshTokenReuse, cacheErr)
}

func (s *Service) Logout(ctx context.Context, refreshToken, accessToken string) error {
	refreshTokenFound := false
	if refreshToken != "" {
		hashed := hashRefreshToken(refreshToken)
		err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
			rt, err := s.store.GetRefreshTokenByHash(txCtx, hashed)
			if errors.Is(err, store.ErrRefreshTokenNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			refreshTokenFound = true
			session, err := s.store.GetSessionByID(txCtx, rt.SessionID)
			if err != nil {
				return err
			}
			if err := s.store.LockUserForUpdate(txCtx, session.UserID); err != nil {
				return err
			}
			rt, err = s.store.GetRefreshTokenByHash(txCtx, hashed)
			if err != nil {
				return err
			}
			if _, err := s.store.GetSessionByID(txCtx, rt.SessionID); err != nil {
				return err
			}
			revokedAt := time.Now().UTC()
			if err := s.accessTokenRevocations.RevokeSession(txCtx, rt.SessionID, revokedAt); err != nil {
				return err
			}
			if err := s.store.RevokeSession(txCtx, rt.SessionID, revokedAt); err != nil {
				return err
			}
			return s.securityEvents.SessionLogout(txCtx, securityevent.Session{
				Outcome:     domain.SecurityEventOutcomeSuccess,
				ActorType:   domain.SecurityEventActorUser,
				ActorUserID: &session.UserID,
				UserID:      &session.UserID,
				SessionID:   &session.ID,
			})
		})
		if err != nil {
			return err
		}
	}

	if refreshTokenFound || accessToken == "" {
		return nil
	}
	now := time.Now().UTC()
	claims, err := s.accessTokens.ParseAny(accessToken, now)
	if errors.Is(err, token.ErrInvalidToken) || errors.Is(err, token.ErrExpiredToken) || errors.Is(err, token.ErrInvalidClaims) {
		// An invalid or expired access token carries no remaining authority.
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.accessTokenRevocations.RevokeToken(ctx, accessToken, claims.ExpiresAt.Time.Sub(now)); err != nil {
		return err
	}
	userID, parseErr := uuid.Parse(claims.Subject)
	if parseErr != nil || userID == uuid.Nil || claims.SessionID == uuid.Nil {
		return nil
	}
	return s.securityEvents.SessionLogout(ctx, securityevent.Session{
		Outcome:        domain.SecurityEventOutcomeSuccess,
		ActorType:      domain.SecurityEventActorUser,
		ActorUserID:    &userID,
		UserID:         &userID,
		SessionID:      &claims.SessionID,
		OrganizationID: &claims.OrgID,
	})
}

func (s *Service) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.LockUserForUpdate(txCtx, userID); err != nil {
			return err
		}
		revokedAt := time.Now().UTC()
		if err := s.accessTokenRevocations.RevokeUser(txCtx, userID, revokedAt); err != nil {
			return err
		}
		if err := s.store.RevokeAllSessionsForUser(txCtx, userID, revokedAt); err != nil {
			return err
		}
		return s.securityEvents.SessionRevoked(txCtx, securityevent.Session{
			Outcome:    domain.SecurityEventOutcomeSuccess,
			ReasonCode: domain.SecurityEventReasonSecurityContainment,
			ActorType:  domain.SecurityEventActorSystem,
			UserID:     &userID,
		})
	})
}

func (s *Service) ValidateAccessToken(
	ctx context.Context,
	accessToken string,
	expectedAudience token.Audience,
	now time.Time,
) (*AccessIdentity, error) {
	claims, err := s.accessTokens.Parse(accessToken, expectedAudience, now)
	if err != nil {
		return nil, err
	}
	if err := s.accessTokenRevocations.Check(ctx, accessToken, claims); err != nil {
		return nil, err
	}

	return s.identityFromClaims(claims)
}

func (s *Service) ValidateAnyAccessToken(
	ctx context.Context,
	accessToken string,
	now time.Time,
) (*AccessIdentity, error) {
	claims, err := s.accessTokens.ParseAny(accessToken, now)
	if err != nil {
		return nil, err
	}
	if err := s.accessTokenRevocations.Check(ctx, accessToken, claims); err != nil {
		return nil, err
	}

	return s.identityFromClaims(claims)
}

func (s *Service) RevokeAccessToken(ctx context.Context, accessToken string, now time.Time) error {
	claims, err := s.accessTokens.ParseAny(accessToken, now)
	if err != nil {
		return err
	}
	return s.accessTokenRevocations.RevokeToken(ctx, accessToken, claims.ExpiresAt.Time.Sub(now))
}

func (s *Service) identityFromClaims(claims *token.AccessClaims) (*AccessIdentity, error) {
	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return nil, token.ErrInvalidToken
	}
	if claims.OrgID == uuid.Nil || claims.OrgRole == "" {
		return nil, token.ErrInvalidToken
	}

	rs, err := roles.FromClaims(claims.Roles)
	if err != nil {
		return nil, err
	}

	return &AccessIdentity{
		UserID:           userID,
		SessionID:        claims.SessionID,
		OrganizationID:   claims.OrgID,
		OrganizationRole: domain.OrganizationRole(claims.OrgRole),
		Roles:            rs,
		EmailVerified:    claims.EmailVerified,
	}, nil
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func shouldRotate(rt domain.RefreshToken, now time.Time, rotation time.Duration) bool {
	switch {
	case rotation < 0:
		return true
	case rotation == 0:
		return false
	default:
		return now.Sub(rt.CreatedAt) >= rotation
	}
}

func (s *Service) ensureUserAllowed(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}

	allowed, err := s.accessPolicy.IsEmailAllowed(ctx, user.Email)
	if err != nil {
		return domain.User{}, err
	}
	if !allowed {
		return domain.User{}, ErrUserNotAllowed
	}
	return user, nil
}

var audienceAccess = map[token.Audience][]roles.Role{
	token.AudienceAdmin: {
		roles.AutharaAdmin,
	},
	token.AudienceOperator: {
		roles.AutharaOperator,
	},
}

func canAccessAudience(rs roles.Roles, audience token.Audience) bool {
	allowed, ok := audienceAccess[audience]
	if !ok {
		return true
	}
	return rs.HasAny(allowed...)
}

func (s *Service) ListUserSessions(
	ctx context.Context,
	userID uuid.UUID,
	currentSessionID uuid.UUID,
	now time.Time,
) ([]domain.Session, error) {
	sessions, err := s.store.ListActiveSessionsByUserID(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	if currentSessionID == uuid.Nil {
		return sessions, nil
	}

	// Put current session first
	for i := range sessions {
		if sessions[i].ID == currentSessionID {
			if i == 0 {
				return sessions, nil
			}
			current := sessions[i]
			out := make([]domain.Session, 0, len(sessions))
			out = append(out, current)
			out = append(out, sessions[:i]...)
			out = append(out, sessions[i+1:]...)
			return out, nil
		}
	}

	return sessions, nil
}

func (s *Service) RevokeUserSession(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	now time.Time,
) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		session, err := s.store.GetSessionByID(txCtx, sessionID)
		if err != nil {
			return err
		}

		// Ownership check is the important security boundary.
		if session.UserID != userID {
			return ErrForbidden
		}
		if err := s.store.LockUserForUpdate(txCtx, userID); err != nil {
			return err
		}
		session, err = s.store.GetSessionByID(txCtx, sessionID)
		if err != nil {
			return err
		}
		if session.UserID != userID {
			return ErrForbidden
		}
		markerAt := time.Now().UTC()
		if now.After(markerAt) {
			markerAt = now
		}
		if err := s.accessTokenRevocations.RevokeSession(txCtx, sessionID, markerAt); err != nil {
			return err
		}
		if err := s.store.RevokeSession(txCtx, sessionID, now); err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensBySession(txCtx, sessionID); err != nil {
			return err
		}
		return s.securityEvents.SessionRevoked(txCtx, securityevent.Session{
			Outcome:     domain.SecurityEventOutcomeSuccess,
			ReasonCode:  domain.SecurityEventReasonUserRequested,
			ActorType:   domain.SecurityEventActorUser,
			ActorUserID: &userID,
			UserID:      &userID,
			SessionID:   &sessionID,
		})
	})
}

func (s *Service) RevokeOtherUserSessions(
	ctx context.Context,
	userID uuid.UUID,
	currentSessionID uuid.UUID,
	now time.Time,
) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.LockUserForUpdate(txCtx, userID); err != nil {
			return err
		}
		sessions, err := s.store.ListActiveSessionsByUserID(txCtx, userID, now)
		if err != nil {
			return err
		}
		markerAt := time.Now().UTC()
		if now.After(markerAt) {
			markerAt = now
		}
		for _, session := range sessions {
			if session.ID != currentSessionID {
				if err := s.accessTokenRevocations.RevokeSession(txCtx, session.ID, markerAt); err != nil {
					return err
				}
				if err := s.securityEvents.SessionRevoked(txCtx, securityevent.Session{
					Outcome:     domain.SecurityEventOutcomeSuccess,
					ReasonCode:  domain.SecurityEventReasonUserRequested,
					ActorType:   domain.SecurityEventActorUser,
					ActorUserID: &userID,
					UserID:      &userID,
					SessionID:   &session.ID,
				}); err != nil {
					return err
				}
			}
		}
		if err := s.store.RevokeOtherSessionsByUserID(txCtx, userID, currentSessionID, now); err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensForOtherSessions(txCtx, userID, currentSessionID); err != nil {
			return err
		}
		return nil
	})
}
