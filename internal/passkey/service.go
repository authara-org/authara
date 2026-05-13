package passkey

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

const defaultChallengeTTL = 5 * time.Minute

type Config struct {
	RPDisplayName string
	RPID          string
	RPOrigins     []string
	Store         *store.Store
	Tx            *tx.Manager
	ChallengeTTL  time.Duration
	Logger        *slog.Logger
}

type Service struct {
	store        *store.Store
	tx           *tx.Manager
	webAuthn     *webauthn.WebAuthn
	challengeTTL time.Duration
	logger       *slog.Logger
}

func New(cfg Config) (*Service, error) {
	challengeTTL := cfg.ChallengeTTL
	if challengeTTL <= 0 {
		challengeTTL = defaultChallengeTTL
	}

	displayName := strings.TrimSpace(cfg.RPDisplayName)
	if displayName == "" {
		displayName = "Authara"
	}

	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: displayName,
		RPID:          cfg.RPID,
		RPOrigins:     cfg.RPOrigins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationRequired,
		},
	})
	if err != nil {
		return nil, err
	}

	return &Service{
		store:        cfg.Store,
		tx:           cfg.Tx,
		webAuthn:     wa,
		challengeTTL: challengeTTL,
		logger:       cfg.Logger,
	}, nil
}

type OptionsResponse struct {
	ChallengeID string          `json:"challenge_id"`
	Options     json.RawMessage `json:"options"`
}

type RegistrationMetadata struct {
	Name         string
	UserAgent    string
	PlatformHint string
}

func (s *Service) BeginRegistration(ctx context.Context, userID uuid.UUID) ([]byte, uuid.UUID, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, uuid.Nil, err
	}

	passkeys, err := s.store.ListPasskeysByUserID(ctx, userID)
	if err != nil {
		return nil, uuid.Nil, err
	}

	waUser := newUser(user, passkeys)
	creation, session, err := s.webAuthn.BeginRegistration(
		waUser,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			RequireResidentKey: protocol.ResidentKeyRequired(),
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		}),
		webauthn.WithExclusions(webauthn.Credentials(waUser.WebAuthnCredentials()).CredentialDescriptors()),
	)
	if err != nil {
		return nil, uuid.Nil, err
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, uuid.Nil, err
	}

	challenge, err := s.store.CreateWebAuthnChallenge(ctx, domain.WebAuthnChallenge{
		UserID:      &userID,
		Purpose:     domain.WebAuthnChallengePurposeRegistration,
		Challenge:   session.Challenge,
		SessionData: sessionJSON,
		ExpiresAt:   time.Now().UTC().Add(s.challengeTTL),
	})
	if err != nil {
		return nil, uuid.Nil, err
	}

	optionsJSON, err := marshalOptions(challenge.ID, creation)
	if err != nil {
		return nil, uuid.Nil, err
	}

	return optionsJSON, challenge.ID, nil
}

func (s *Service) FinishRegistration(
	ctx context.Context,
	userID uuid.UUID,
	challengeID uuid.UUID,
	credentialResponseJSON []byte,
	metadata RegistrationMetadata,
) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		now := time.Now().UTC()
		challenge, session, err := s.registrationSession(txCtx, userID, challengeID, now)
		if err != nil {
			return err
		}

		user, err := s.store.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}

		passkeys, err := s.store.ListPasskeysByUserID(txCtx, userID)
		if err != nil {
			return err
		}

		credential, err := s.webAuthn.FinishRegistration(
			newUser(user, passkeys),
			session,
			webAuthnRequest(credentialResponseJSON),
		)
		if err != nil {
			s.logPasskeyFailure(ctx, "finish passkey registration failed", err)
			return ErrPasskeyRegistrationInvalid
		}
		if !credential.Flags.UserVerified {
			return ErrPasskeyRegistrationInvalid
		}

		passkeyName := passkeyRegistrationName(metadata, credential, now)
		_, err = s.store.CreatePasskey(txCtx, domainFromCredential(userID, credential, passkeyName))
		if err != nil {
			if errors.Is(err, store.ErrPasskeyAlreadyExists) ||
				store.IsUniqueViolation(err, store.ConstraintPasskeyCredentialID) {
				return ErrPasskeyAlreadyExists
			}
			return err
		}

		return s.store.ConsumeWebAuthnChallenge(txCtx, challenge.ID, now)
	})
}

func (s *Service) BeginLogin(ctx context.Context) ([]byte, uuid.UUID, error) {
	assertion, session, err := s.webAuthn.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, uuid.Nil, err
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return nil, uuid.Nil, err
	}

	challenge, err := s.store.CreateWebAuthnChallenge(ctx, domain.WebAuthnChallenge{
		Purpose:     domain.WebAuthnChallengePurposeAuthentication,
		Challenge:   session.Challenge,
		SessionData: sessionJSON,
		ExpiresAt:   time.Now().UTC().Add(s.challengeTTL),
	})
	if err != nil {
		return nil, uuid.Nil, err
	}

	optionsJSON, err := marshalOptions(challenge.ID, assertion)
	if err != nil {
		return nil, uuid.Nil, err
	}

	return optionsJSON, challenge.ID, nil
}

func (s *Service) FinishLogin(
	ctx context.Context,
	challengeID uuid.UUID,
	assertionResponseJSON []byte,
	now time.Time,
) (domain.User, error) {
	var out domain.User

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		challenge, session, err := s.authenticationSession(txCtx, challengeID, now)
		if err != nil {
			return err
		}

		validatedUser, credential, err := s.webAuthn.FinishPasskeyLogin(
			func(rawID, userHandle []byte) (webauthn.User, error) {
				return s.lookupUserByCredential(txCtx, rawID)
			},
			session,
			webAuthnRequest(assertionResponseJSON),
		)
		if err != nil {
			s.logPasskeyFailure(ctx, "finish passkey login failed", err)
			return ErrPasskeyAuthenticationInvalid
		}
		if !credential.Flags.UserVerified {
			return ErrPasskeyAuthenticationInvalid
		}

		waUser, ok := validatedUser.(*user)
		if !ok {
			return ErrPasskeyAuthenticationInvalid
		}
		out = waUser.user

		if err := s.store.UpdatePasskeyAfterLogin(
			txCtx,
			credential.ID,
			credential.Authenticator.SignCount,
			credential.Authenticator.CloneWarning,
			now,
		); err != nil {
			return err
		}

		return s.store.ConsumeWebAuthnChallenge(txCtx, challenge.ID, now)
	})
	if err != nil {
		return domain.User{}, err
	}

	return out, nil
}

func (s *Service) ListUserPasskeys(ctx context.Context, userID uuid.UUID) ([]domain.Passkey, error) {
	return s.store.ListPasskeysByUserID(ctx, userID)
}

func (s *Service) DeletePasskey(ctx context.Context, userID uuid.UUID, passkeyID uuid.UUID) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.LockUserForAuthMethodMutation(txCtx, userID); err != nil {
			return err
		}

		count, err := s.store.CountAuthMethods(txCtx, userID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrCannotRemoveLastAuthMethod
		}

		err = s.store.DeletePasskeyByIDAndUserID(txCtx, passkeyID, userID)
		if err != nil {
			if errors.Is(err, store.ErrPasskeyNotFound) {
				return ErrPasskeyNotFound
			}
			return err
		}

		return nil
	})
}

func (s *Service) registrationSession(
	ctx context.Context,
	userID uuid.UUID,
	challengeID uuid.UUID,
	now time.Time,
) (domain.WebAuthnChallenge, webauthn.SessionData, error) {
	challenge, session, err := s.loadChallengeSession(ctx, challengeID, now)
	if err != nil {
		if errors.Is(err, ErrPasskeyAuthenticationInvalid) {
			return domain.WebAuthnChallenge{}, webauthn.SessionData{}, ErrPasskeyRegistrationInvalid
		}
		return domain.WebAuthnChallenge{}, webauthn.SessionData{}, err
	}
	if challenge.Purpose != domain.WebAuthnChallengePurposeRegistration ||
		challenge.UserID == nil ||
		*challenge.UserID != userID {
		return domain.WebAuthnChallenge{}, webauthn.SessionData{}, ErrPasskeyRegistrationInvalid
	}
	return challenge, session, nil
}

func (s *Service) authenticationSession(
	ctx context.Context,
	challengeID uuid.UUID,
	now time.Time,
) (domain.WebAuthnChallenge, webauthn.SessionData, error) {
	challenge, session, err := s.loadChallengeSession(ctx, challengeID, now)
	if err != nil {
		return domain.WebAuthnChallenge{}, webauthn.SessionData{}, err
	}
	if challenge.Purpose != domain.WebAuthnChallengePurposeAuthentication {
		return domain.WebAuthnChallenge{}, webauthn.SessionData{}, ErrPasskeyAuthenticationInvalid
	}
	return challenge, session, nil
}

func (s *Service) loadChallengeSession(
	ctx context.Context,
	challengeID uuid.UUID,
	now time.Time,
) (domain.WebAuthnChallenge, webauthn.SessionData, error) {
	challenge, err := s.store.GetWebAuthnChallengeByIDForUpdate(ctx, challengeID)
	if err != nil {
		if errors.Is(err, store.ErrWebAuthnChallengeNotFound) {
			return domain.WebAuthnChallenge{}, webauthn.SessionData{}, ErrPasskeyAuthenticationInvalid
		}
		return domain.WebAuthnChallenge{}, webauthn.SessionData{}, err
	}
	if challenge.ConsumedAt != nil || !challenge.ExpiresAt.After(now) {
		return domain.WebAuthnChallenge{}, webauthn.SessionData{}, ErrPasskeyAuthenticationInvalid
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(challenge.SessionData, &session); err != nil {
		return domain.WebAuthnChallenge{}, webauthn.SessionData{}, err
	}

	return challenge, session, nil
}

func (s *Service) lookupUserByCredential(ctx context.Context, rawID []byte) (webauthn.User, error) {
	passkey, err := s.store.GetPasskeyByCredentialID(ctx, rawID)
	if err != nil {
		return nil, err
	}

	u, err := s.store.GetUserByID(ctx, passkey.UserID)
	if err != nil {
		return nil, err
	}

	passkeys, err := s.store.ListPasskeysByUserID(ctx, passkey.UserID)
	if err != nil {
		return nil, err
	}

	return newUser(u, passkeys), nil
}

func (s *Service) logPasskeyFailure(ctx context.Context, message string, err error) {
	if s.logger == nil {
		return
	}
	s.logger.WarnContext(ctx, message, "err", err)
}

func marshalOptions(challengeID uuid.UUID, options any) ([]byte, error) {
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(OptionsResponse{
		ChallengeID: challengeID.String(),
		Options:     optionsJSON,
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func webAuthnRequest(body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Passkey"
	}
	if utf8.RuneCountInString(name) > 255 {
		return string([]rune(name)[:255])
	}
	return name
}

func passkeyRegistrationName(
	metadata RegistrationMetadata,
	credential *webauthn.Credential,
	createdAt time.Time,
) string {
	if strings.TrimSpace(metadata.Name) != "" {
		return sanitizeName(metadata.Name)
	}

	return sanitizeName(fmt.Sprintf(
		"%s • %s • %s",
		passkeyPlatformName(metadata, credential),
		passkeyAuthenticatorType(metadata, credential),
		createdAt.Format("Jan 2006"),
	))
}

func passkeyPlatformName(metadata RegistrationMetadata, credential *webauthn.Credential) string {
	hint := strings.ToLower(strings.TrimSpace(metadata.PlatformHint))
	ua := strings.ToLower(metadata.UserAgent)

	if isSecurityKeyCredential(credential) {
		return securityKeyPlatformName(credential)
	}

	switch {
	case containsAny(hint, "iphone") || containsAny(ua, "iphone"):
		return "iPhone"
	case containsAny(hint, "ipad") || containsAny(ua, "ipad"):
		return "iPad"
	case hint == "ios":
		return "iPhone"
	case containsAny(hint, "macos", "mac os", "mac") ||
		containsAny(ua, "macintosh", "mac os x"):
		return "MacBook Pro"
	case containsAny(hint, "chrome os", "chromium os", "cros") || containsAny(ua, "cros"):
		return "Chromebook"
	case containsAny(hint, "windows") || containsAny(ua, "windows"):
		return "Windows PC"
	case containsAny(hint, "android") || containsAny(ua, "android"):
		return "Android Phone"
	case containsAny(hint, "linux") || containsAny(ua, "linux"):
		return "Linux PC"
	default:
		return "This Device"
	}
}

func passkeyAuthenticatorType(metadata RegistrationMetadata, credential *webauthn.Credential) string {
	if credential == nil {
		return "Passkey"
	}
	if isSecurityKeyCredential(credential) {
		return "Security Key"
	}

	hint := strings.ToLower(strings.TrimSpace(metadata.PlatformHint))
	ua := strings.ToLower(metadata.UserAgent)

	if credential.Authenticator.Attachment == protocol.Platform ||
		hasTransport(credential, protocol.Internal) {
		switch {
		case containsAny(hint, "windows") || containsAny(ua, "windows"):
			return "Windows Hello"
		case containsAny(hint, "iphone", "ipad") || hint == "ios" ||
			containsAny(ua, "iphone", "ipad"):
			return "Face ID"
		case containsAny(hint, "macos", "mac os", "mac") ||
			containsAny(ua, "macintosh", "mac os x"):
			return "Touch ID"
		case containsAny(hint, "android") || containsAny(ua, "android"):
			return "Android Biometrics"
		case containsAny(hint, "chrome os", "chromium os", "cros") || containsAny(ua, "cros"):
			return "ChromeOS Biometrics"
		default:
			return "Device Passkey"
		}
	}

	if hasTransport(credential, protocol.Hybrid) {
		return "Synced Passkey"
	}

	return "Passkey"
}

func isSecurityKeyCredential(credential *webauthn.Credential) bool {
	if credential == nil {
		return false
	}
	if hasTransport(credential, protocol.USB, protocol.NFC, protocol.BLE, protocol.SmartCard) {
		return true
	}
	return credential.Authenticator.Attachment == protocol.CrossPlatform &&
		!hasTransport(credential, protocol.Hybrid, protocol.Internal)
}

func hasTransport(credential *webauthn.Credential, transports ...protocol.AuthenticatorTransport) bool {
	if credential == nil {
		return false
	}
	for _, actual := range credential.Transport {
		for _, expected := range transports {
			if actual == expected {
				return true
			}
		}
	}
	return false
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

var yubicoAAGUIDNames = map[string]string{
	"cb69481e-8ff7-4039-93ec-0a2729a154a8": "YubiKey 5",
	"ee882879-721c-4913-9775-3dfcce97072a": "YubiKey 5",
	"fa2b99dc-9e39-4257-8f92-4a30d23c4118": "YubiKey 5 NFC",
	"2fc0579f-8113-47ea-b116-bb5a8db9202a": "YubiKey 5 NFC",
	"1ac71f64-468d-4fe0-bef1-0e5f2f551f18": "YubiKey 5 NFC",
	"6ab56fad-881f-4a43-acb2-0be065924522": "YubiKey 5 NFC",
	"b2c1a50b-dad8-4dc7-ba4d-0ce9597904bc": "YubiKey 5 NFC",
	"20ac7a17-c814-4833-93fe-539f0d5e3389": "YubiKey 5 Series",
	"4599062e-6926-4fe7-9566-9e8fb1aedaa0": "YubiKey 5 Series",
}

func securityKeyPlatformName(credential *webauthn.Credential) string {
	if credential == nil || len(credential.Authenticator.AAGUID) != 16 {
		return "Security Key"
	}

	aaguid, err := uuid.FromBytes(credential.Authenticator.AAGUID)
	if err != nil {
		return "Security Key"
	}
	if name, ok := yubicoAAGUIDNames[aaguid.String()]; ok {
		return name
	}

	return "Security Key"
}

func domainFromCredential(userID uuid.UUID, credential *webauthn.Credential, name string) domain.Passkey {
	var aaguid *uuid.UUID
	if len(credential.Authenticator.AAGUID) == 16 {
		if parsed, err := uuid.FromBytes(credential.Authenticator.AAGUID); err == nil && parsed != uuid.Nil {
			aaguid = &parsed
		}
	}

	return domain.Passkey{
		UserID:            userID,
		CredentialID:      credential.ID,
		PublicKey:         credential.PublicKey,
		AttestationType:   credential.AttestationType,
		AttestationFormat: credential.AttestationFormat,
		Transport:         transportsToStrings(credential.Transport),
		AAGUID:            aaguid,
		SignCount:         credential.Authenticator.SignCount,
		CloneWarning:      credential.Authenticator.CloneWarning,
		Name:              name,
		UserPresent:       credential.Flags.UserPresent,
		UserVerified:      credential.Flags.UserVerified,
		BackupEligible:    credential.Flags.BackupEligible,
		BackupState:       credential.Flags.BackupState,
	}
}

func credentialFromDomain(passkey domain.Passkey) webauthn.Credential {
	var aaguid []byte
	if passkey.AAGUID != nil {
		aaguid, _ = passkey.AAGUID.MarshalBinary()
	}

	return webauthn.Credential{
		ID:                passkey.CredentialID,
		PublicKey:         passkey.PublicKey,
		AttestationType:   passkey.AttestationType,
		AttestationFormat: passkey.AttestationFormat,
		Transport:         transportsFromStrings(passkey.Transport),
		Flags: webauthn.CredentialFlags{
			UserPresent:    passkey.UserPresent,
			UserVerified:   passkey.UserVerified,
			BackupEligible: passkey.BackupEligible,
			BackupState:    passkey.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       aaguid,
			SignCount:    passkey.SignCount,
			CloneWarning: passkey.CloneWarning,
		},
	}
}

func transportsToStrings(in []protocol.AuthenticatorTransport) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		if t != "" {
			out = append(out, string(t))
		}
	}
	return out
}

func transportsFromStrings(in []string) []protocol.AuthenticatorTransport {
	out := make([]protocol.AuthenticatorTransport, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, protocol.AuthenticatorTransport(t))
		}
	}
	return out
}

type user struct {
	user        domain.User
	credentials []webauthn.Credential
}

func newUser(u domain.User, passkeys []domain.Passkey) *user {
	credentials := make([]webauthn.Credential, 0, len(passkeys))
	for _, passkey := range passkeys {
		credentials = append(credentials, credentialFromDomain(passkey))
	}
	return &user{user: u, credentials: credentials}
}

func (u *user) WebAuthnID() []byte {
	id, err := u.user.ID.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("marshal user id: %v", err))
	}
	return id
}

func (u *user) WebAuthnName() string {
	return u.user.Email
}

func (u *user) WebAuthnDisplayName() string {
	if strings.TrimSpace(u.user.Username) != "" {
		return u.user.Username
	}
	return u.user.Email
}

func (u *user) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}
