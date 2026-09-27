package securityevent

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestTypedRecorderHonorsEnabledEvents(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		recorder := New(Config{
			Store: tdb.Store,
			EnabledEvents: map[domain.SecurityEventType]struct{}{
				domain.SecurityEventAuthenticationLogin: {},
			},
		})
		userID := uuid.New()
		sessionID := uuid.New()
		if err := recorder.SessionRefresh(ctx, Session{
			Outcome: domain.SecurityEventOutcomeSuccess, ActorType: domain.SecurityEventActorUser,
			ActorUserID: &userID, UserID: &userID, SessionID: &sessionID,
		}); err != nil {
			t.Fatal(err)
		}
		if err := recorder.AuthenticationLogin(ctx, Authentication{
			Outcome: domain.SecurityEventOutcomeDenied, ReasonCode: domain.SecurityEventReasonInvalidCredentials,
			ActorType: domain.SecurityEventActorAnonymous, AuthenticationMethod: domain.AuthenticationMethodPassword,
		}); err != nil {
			t.Fatal(err)
		}

		events, err := recorder.Query(ctx, Filter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].Type != domain.SecurityEventAuthenticationLogin {
			t.Fatalf("unexpected enabled events: %+v", events)
		}
	})
}

func TestDisabledEventDoesNotRequireStore(t *testing.T) {
	recorder := New(Config{})
	if err := recorder.SessionRefresh(context.Background(), Session{
		Outcome:   domain.SecurityEventOutcomeSuccess,
		ActorType: domain.SecurityEventActorUser,
	}); err != nil {
		t.Fatalf("disabled event touched storage: %v", err)
	}
}

func TestEnabledEventRequiresStore(t *testing.T) {
	recorder := New(Config{EnabledEvents: map[domain.SecurityEventType]struct{}{
		domain.SecurityEventAuthenticationLogin: {},
	}})
	err := recorder.AuthenticationLogin(context.Background(), Authentication{
		Outcome:   domain.SecurityEventOutcomeDenied,
		ActorType: domain.SecurityEventActorAnonymous,
	})
	if !errors.Is(err, ErrStoreRequired) {
		t.Fatalf("error = %v, want ErrStoreRequired", err)
	}
}

func TestCredentialMethodBuildsCanonicalSafeEvent(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		recorder := NewStandard(tdb.Store, 180*24*time.Hour)
		userID := uuid.New()
		sessionID := uuid.New()
		if err := recorder.CredentialPasswordChanged(ctx, Credential{UserID: userID, SessionID: &sessionID}); err != nil {
			t.Fatal(err)
		}
		events, err := recorder.Query(ctx, Filter{Type: domain.SecurityEventCredentialPasswordChanged, UserID: &userID})
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("events = %d, want 1", len(events))
		}
		got := events[0]
		if got.Outcome != domain.SecurityEventOutcomeSuccess || got.ActorType != domain.SecurityEventActorUser ||
			got.ActorUserID == nil || *got.ActorUserID != userID || got.SessionID == nil || *got.SessionID != sessionID ||
			got.Response != "" || got.ReasonCode != "" {
			t.Fatalf("unexpected canonical event: %+v", got)
		}
	})
}

func TestExportNDJSONUsesSafeShape(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		recorder := NewStandard(tdb.Store, 180*24*time.Hour)
		userID := uuid.New()
		sessionID := uuid.New()
		if err := recorder.AuthenticationLogin(ctx, Authentication{
			Outcome: domain.SecurityEventOutcomeSuccess, ActorType: domain.SecurityEventActorUser,
			ActorUserID: &userID, UserID: &userID, SessionID: &sessionID,
			AuthenticationMethod: domain.AuthenticationMethodPassword,
		}); err != nil {
			t.Fatal(err)
		}

		var out bytes.Buffer
		if err := recorder.ExportNDJSON(ctx, store.SecurityEventFilter{SessionID: &sessionID}, &out); err != nil {
			t.Fatal(err)
		}
		exported := out.String()
		if !strings.HasSuffix(exported, "\n") || !strings.Contains(exported, `"type":"authentication.login"`) {
			t.Fatalf("unexpected NDJSON export: %q", exported)
		}
		for _, forbidden := range []string{`"email"`, `"ip"`, `"user_agent"`, `"metadata"`} {
			if strings.Contains(exported, forbidden) {
				t.Fatalf("export contains forbidden field %s: %s", forbidden, exported)
			}
		}
	})
}
