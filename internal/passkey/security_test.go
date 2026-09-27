package passkey

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/securityevent"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

func TestPasskeyCounterRegressionCreatesOneSafeSecurityAlert(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		service := newSecurityTestService(t, tdb, config.AuthenticationPolicy{
			PasskeyCloneResponse:   config.PasskeyCloneResponseAlert,
			PasskeyCloneNotifyUser: true,
		})
		user, passkey := createSecurityTestPasskey(t, ctx, tdb, 7)
		now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

		authenticator := webauthn.Authenticator{SignCount: passkey.SignCount}
		authenticator.UpdateCounter(6)
		credential := &webauthn.Credential{ID: passkey.CredentialID, Authenticator: authenticator}

		decision, err := service.applyAuthenticationResult(ctx, user, passkey, credential, now)
		if err != nil {
			t.Fatalf("applyAuthenticationResult failed: %v", err)
		}
		if !decision.AllowSession {
			t.Fatalf("unexpected alert decision: %+v", decision)
		}

		stored, err := tdb.Store.GetPasskeyByCredentialID(ctx, passkey.CredentialID)
		if err != nil {
			t.Fatal(err)
		}
		if !stored.CloneWarning || stored.SignCount != 7 || stored.RestrictedAt != nil {
			t.Fatalf("unexpected stored passkey: %+v", stored)
		}

		events, err := tdb.Store.ListSecurityEvents(ctx, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].Type != domain.SecurityEventPasskeyCloneWarning || events[0].Response != config.PasskeyCloneResponseAlert {
			t.Fatalf("unexpected security events: %+v", events)
		}

		data := testutil.LatestEmailTemplateData(t, ctx, user.Email, domain.EmailTemplateSuspiciousPasskeyActivity)
		if len(data) != 1 || data[email.TemplateVariableOccurredAt] != email.OccurredAt(now) {
			t.Fatalf("notification contains unexpected data: %#v", data)
		}

		decision, err = service.applyAuthenticationResult(ctx, user, stored, credential, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("repeat applyAuthenticationResult failed: %v", err)
		}
		if !decision.AllowSession {
			t.Fatalf("repeat alert unexpectedly denied session: %+v", decision)
		}
		events, err = tdb.Store.ListSecurityEvents(ctx, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("security event count = %d, want 1", len(events))
		}
	})
}

func TestPasskeyZeroCounterDoesNotCreateSecurityEvent(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		service := newSecurityTestService(t, tdb, config.AuthenticationPolicy{
			PasskeyCloneResponse:   config.PasskeyCloneResponseRestrictAndRevoke,
			PasskeyCloneNotifyUser: true,
		})
		user, passkey := createSecurityTestPasskey(t, ctx, tdb, 0)
		authenticator := webauthn.Authenticator{}
		authenticator.UpdateCounter(0)

		decision, err := service.applyAuthenticationResult(ctx, user, passkey, &webauthn.Credential{
			ID: passkey.CredentialID, Authenticator: authenticator,
		}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if !decision.AllowSession {
			t.Fatalf("zero counter was treated as suspicious: %+v", decision)
		}
		events, err := tdb.Store.ListSecurityEvents(ctx, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 0 {
			t.Fatalf("zero counter created security events: %+v", events)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateSuspiciousPasskeyActivity); got != 0 {
			t.Fatalf("zero counter queued %d notifications", got)
		}
	})
}

func TestTightenedClonePolicyRestrictsNextAssertion(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		alertService := newSecurityTestService(t, tdb, config.AuthenticationPolicy{
			PasskeyCloneResponse: config.PasskeyCloneResponseAlert,
		})
		user, passkey := createSecurityTestPasskey(t, ctx, tdb, 7)
		credential := &webauthn.Credential{
			ID:            passkey.CredentialID,
			Authenticator: webauthn.Authenticator{SignCount: 7, CloneWarning: true},
		}
		if _, err := alertService.applyAuthenticationResult(ctx, user, passkey, credential, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}

		stored, err := tdb.Store.GetPasskeyByCredentialID(ctx, passkey.CredentialID)
		if err != nil {
			t.Fatal(err)
		}
		restrictService := newSecurityTestService(t, tdb, config.AuthenticationPolicy{
			PasskeyCloneResponse: config.PasskeyCloneResponseRestrict,
		})
		decision, err := restrictService.applyAuthenticationResult(ctx, user, stored, credential, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if decision.AllowSession {
			t.Fatal("credential remained usable after clone policy was tightened")
		}
		stored, err = tdb.Store.GetPasskeyByCredentialID(ctx, passkey.CredentialID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.RestrictedAt == nil {
			t.Fatal("existing clone warning was not restricted")
		}
		events, err := tdb.Store.ListSecurityEvents(ctx, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		responses := map[string]bool{}
		for _, event := range events {
			responses[event.Response] = true
		}
		if len(events) != 2 || !responses[config.PasskeyCloneResponseAlert] || !responses[config.PasskeyCloneResponseRestrict] {
			t.Fatalf("unexpected security events: %+v", events)
		}
	})
}

func TestPasskeyRestrictAndRevokeDecisionPersistsRestriction(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		revoker := session.New(session.SessionConfig{Store: tdb.Store, Tx: tdb.Tx})
		service := newSecurityTestService(t, tdb, config.AuthenticationPolicy{
			PasskeyCloneResponse: config.PasskeyCloneResponseRestrictAndRevoke,
		}, revoker)
		user, passkey := createSecurityTestPasskey(t, ctx, tdb, 3)
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		sessionRow, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID: user.ID, ActiveOrganizationID: org.ID, ExpiresAt: time.Now().UTC().Add(time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := tdb.Store.LockUserForUpdate(ctx, user.ID); err != nil {
			t.Fatal(err)
		}

		decision, err := service.applyAuthenticationResult(ctx, user, passkey, &webauthn.Credential{
			ID:            passkey.CredentialID,
			Authenticator: webauthn.Authenticator{SignCount: 3, CloneWarning: true},
		}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if decision.AllowSession {
			t.Fatalf("unexpected restrictive decision: %+v", decision)
		}
		stored, err := tdb.Store.GetPasskeyByCredentialID(ctx, passkey.CredentialID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.RestrictedAt == nil {
			t.Fatal("passkey was not restricted")
		}
		storedSession, err := tdb.Store.GetSessionByID(ctx, sessionRow.ID)
		if err != nil {
			t.Fatal(err)
		}
		if storedSession.RevokedAt == nil {
			t.Fatal("existing session was not revoked in the containment transaction")
		}
	})
}

func TestPasskeyRevocationFailureRollsBackContainment(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	user, passkey := createSecurityTestPasskey(t, ctx, tdb, 4)
	t.Cleanup(func() { _ = tdb.Store.DeleteUser(context.Background(), user.ID) })
	revokeErr := errors.New("revocation unavailable")
	service := newSecurityTestService(t, tdb, config.AuthenticationPolicy{
		PasskeyCloneResponse: config.PasskeyCloneResponseRestrictAndRevoke,
	}, &testSessionRevoker{err: revokeErr})

	err := service.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		_, err := service.applyAuthenticationResult(txCtx, user, passkey, &webauthn.Credential{
			ID:            passkey.CredentialID,
			Authenticator: webauthn.Authenticator{SignCount: 4, CloneWarning: true},
		}, time.Now().UTC())
		return err
	})
	if !errors.Is(err, revokeErr) {
		t.Fatalf("expected revocation failure, got %v", err)
	}
	stored, err := tdb.Store.GetPasskeyByCredentialID(ctx, passkey.CredentialID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CloneWarning || stored.RestrictedAt != nil {
		t.Fatalf("containment committed after revocation failure: %+v", stored)
	}
	events, err := tdb.Store.ListSecurityEvents(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("security event committed after revocation failure: %+v", events)
	}
}

type testSessionRevoker struct {
	err error
}

func (r *testSessionRevoker) RevokeAllSessions(context.Context, uuid.UUID) error {
	return r.err
}

func newSecurityTestService(t *testing.T, tdb *testutil.TestDB, policy config.AuthenticationPolicy, revokers ...SessionRevoker) *Service {
	t.Helper()
	revoker := SessionRevoker(&testSessionRevoker{})
	if len(revokers) > 0 {
		revoker = revokers[0]
	}
	service, err := New(Config{
		RPDisplayName: "Authara",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:3000"},
		Store:         tdb.Store,
		Tx:            tdb.Tx,
		Policy: config.AuthenticationPolicyReaderFunc(func() config.AuthenticationPolicy {
			return policy
		}),
		SessionRevoker: revoker,
		SecurityEvents: securityevent.NewStandard(tdb.Store, 180*24*time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func createSecurityTestPasskey(t *testing.T, ctx context.Context, tdb *testutil.TestDB, signCount uint32) (domain.User, domain.Passkey) {
	t.Helper()
	suffix := uuid.NewString()
	user, err := tdb.Store.CreateUser(ctx, domain.User{Email: suffix + "@example.com", Username: suffix})
	if err != nil {
		t.Fatal(err)
	}
	passkey, err := tdb.Store.CreatePasskey(ctx, domain.Passkey{
		UserID: user.ID, CredentialID: []byte("credential-" + suffix), PublicKey: []byte("public-key"), SignCount: signCount,
	})
	if err != nil {
		t.Fatal(err)
	}
	return user, passkey
}
