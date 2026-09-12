package config

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewServiceRequiresStartupConfigurationAndStore(t *testing.T) {
	if _, err := NewService(context.Background(), ServiceOptions{}); err == nil || !strings.Contains(err.Error(), "startup config is required") {
		t.Fatalf("missing startup config error = %v", err)
	}
	if _, err := NewService(context.Background(), ServiceOptions{Startup: &Config{}}); err == nil || !strings.Contains(err.Error(), "runtime settings store is required") {
		t.Fatalf("missing runtime settings store error = %v", err)
	}
}

func TestResolutionPrecedenceAndEnvironmentLocking(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	store.seed(KeyChallengeTTL, `"45m"`, 3)

	defaults, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatalf("New defaults: %v", err)
	}
	if got := defaults.Current(); got.TTL != 45*time.Minute || got.MaxAttempts != 5 {
		t.Fatalf("operator/default policy = %+v", got)
	}
	described, err := defaults.Describe(KeyChallengeTTL)
	if err != nil {
		t.Fatal(err)
	}
	if described.EffectiveSource != SourceOperator || described.Locked || described.Revision != 3 {
		t.Fatalf("operator description = %+v", described)
	}

	locked, err := NewService(ctx, ServiceOptions{Startup: &Config{},
		Store: store,
		LookupEnvironment: environment(map[string]string{
			"AUTHARA_CHALLENGE_TTL": "1h",
		}),
	})
	if err != nil {
		t.Fatalf("New environment override: %v", err)
	}
	described, _ = locked.Describe(KeyChallengeTTL)
	if locked.Current().TTL != time.Hour || described.EffectiveSource != SourceEnvironment ||
		!described.Locked || !described.DormantOverride || described.PersistedOverride == nil ||
		*described.PersistedOverride != "45m0s" {
		t.Fatalf("environment description = %+v, policy = %+v", described, locked.Current())
	}
	if _, err := locked.Set(ctx, KeyChallengeTTL, "2h", uuid.New(), 3); !errors.Is(err, ErrSettingLocked) {
		t.Fatalf("locked Set error = %v", err)
	}
	if _, err := locked.Clear(ctx, KeyChallengeTTL, uuid.New(), 3); !errors.Is(err, ErrSettingLocked) {
		t.Fatalf("locked Clear error = %v", err)
	}

	// Lookup returning false is authoritative: parser-provided defaults are not
	// interpreted as an explicit environment lock.
	absent, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: newMemoryStore(), LookupEnvironment: func(string) (string, bool) {
		return "30m", false
	}})
	if err != nil {
		t.Fatal(err)
	}
	described, _ = absent.Describe(KeyChallengeTTL)
	if described.EffectiveSource != SourceDefault || described.Locked {
		t.Fatalf("absent environment description = %+v", described)
	}
}

func TestEnvironmentCatalogReportsSetDefaultUnsetAndSensitiveStates(t *testing.T) {
	variables := []EnvironmentVariable{
		{Name: "APP_ENV", DisplayName: "Application environment", Group: "Runtime", Type: "enum", Default: "dev", HasDefault: true},
		{Name: "LOG_LEVEL", DisplayName: "Log level", Group: "Runtime", Type: "string"},
		{Name: "PUBLIC_URL", DisplayName: "Public URL", Group: "Public URL", Type: "url", Required: true},
		{Name: "AUTHARA_JWT_KEYS", DisplayName: "JWT keys", Group: "Tokens", Type: "map", Required: true, Sensitive: true},
		{Name: "AUTHARA_INTERNAL_API_TOKEN", DisplayName: "Internal API token", Group: "Internal API", Type: "string", Sensitive: true},
		{Name: "AUTHARA_OAUTH_GOOGLE_CLIENT_ID", DisplayName: "OAuth Google client ID", Group: "OAuth", Type: "string"},
		{Name: "AUTHARA_CHALLENGE_TTL", DisplayName: "Challenge TTL", Group: "Challenges", Type: "duration", Default: "30m", HasDefault: true},
	}
	service, err := NewService(context.Background(), ServiceOptions{Startup: &Config{},
		Store:       newMemoryStore(),
		Environment: variables,
		LookupEnvironment: environment(map[string]string{
			"PUBLIC_URL":                 "https://auth.example",
			"AUTHARA_JWT_KEYS":           "sensitive-key-material",
			"AUTHARA_INTERNAL_API_TOKEN": "",
			"AUTHARA_CHALLENGE_TTL":      "45m",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	descriptions := make(map[string]Description)
	for _, description := range service.List() {
		descriptions[description.Environment] = description
	}
	assertEnvironmentDescription(t, descriptions["APP_ENV"], SourceDefault, "dev", true)
	assertEnvironmentDescription(t, descriptions["LOG_LEVEL"], SourceDefault, "debug", true)
	assertEnvironmentDescription(t, descriptions["PUBLIC_URL"], SourceEnvironment, "https://auth.example", true)
	assertEnvironmentDescription(t, descriptions["AUTHARA_OAUTH_GOOGLE_CLIENT_ID"], SourceUnset, "", true)
	assertEnvironmentDescription(t, descriptions["AUTHARA_INTERNAL_API_TOKEN"], SourceUnset, "", true)
	assertEnvironmentDescription(t, descriptions["AUTHARA_CHALLENGE_TTL"], SourceEnvironment, "45m0s", true)

	secret := descriptions["AUTHARA_JWT_KEYS"]
	if secret.EffectiveSource != SourceEnvironment || secret.EffectiveValue != "Configured (value hidden)" || !secret.Locked {
		t.Fatalf("sensitive description = %+v", secret)
	}
	if got := service.environmentValues[Key("AUTHARA_JWT_KEYS")]; got != "" {
		t.Fatalf("runtime settings retained sensitive value: %q", got)
	}
	if editable := descriptions["AUTHARA_CHALLENGE_MAX_ATTEMPTS"]; editable.Locked || editable.EffectiveSource != SourceDefault {
		t.Fatalf("operator-editable description = %+v", editable)
	}
}

func TestEnvironmentCatalogResolvesProductionLogLevelDefault(t *testing.T) {
	service, err := NewService(context.Background(), ServiceOptions{Startup: &Config{},
		Store: newMemoryStore(),
		Environment: []EnvironmentVariable{
			{Name: "APP_ENV", DisplayName: "Application environment", Group: "Runtime", Type: "enum", Default: "dev", HasDefault: true},
			{Name: "LOG_LEVEL", DisplayName: "Log level", Group: "Runtime", Type: "string"},
		},
		LookupEnvironment: environment(map[string]string{"APP_ENV": "prod", "LOG_LEVEL": ""}),
	})
	if err != nil {
		t.Fatal(err)
	}
	var logLevel Description
	for _, description := range service.List() {
		if description.Environment == "LOG_LEVEL" {
			logLevel = description
			break
		}
	}
	assertEnvironmentDescription(t, logLevel, SourceDefault, "info", true)
}

func TestRateLimitPolicySupportsLiveOverridesAndEnvironmentLocks(t *testing.T) {
	store := newMemoryStore()
	service, err := NewService(context.Background(), ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if got := service.CurrentRateLimits(); got.LoginIPLimit != 5 || got.LoginIPWindow != time.Minute || got.MaxEntries != 50_000 {
		t.Fatalf("default rate-limit policy = %+v", got)
	}

	actor := uuid.New()
	updated, err := service.Set(context.Background(), KeyRateLimitLoginIPLimit, "12", actor, 0)
	if err != nil {
		t.Fatalf("set login IP limit: %v", err)
	}
	if got := service.CurrentRateLimits().LoginIPLimit; got != 12 {
		t.Fatalf("live login IP limit = %d, want 12", got)
	}
	if updated.EffectiveSource != SourceOperator || updated.Locked {
		t.Fatalf("updated rate-limit description = %+v", updated)
	}
	restarted, err := NewService(context.Background(), ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if got := restarted.CurrentRateLimits().LoginIPLimit; got != 12 {
		t.Fatalf("persisted login IP limit after reconstruction = %d, want 12", got)
	}
	if _, err := service.Clear(context.Background(), KeyRateLimitLoginIPLimit, actor, updated.Revision); err != nil {
		t.Fatalf("clear login IP limit: %v", err)
	}
	if got := service.CurrentRateLimits().LoginIPLimit; got != 5 {
		t.Fatalf("cleared login IP limit = %d, want default 5", got)
	}
	if _, err := service.Set(context.Background(), KeyRateLimitLoginIPWindow, "500ms", actor, 0); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("unsafe rate-limit window error = %v", err)
	}

	locked, err := NewService(context.Background(), ServiceOptions{Startup: &Config{},
		Store: newMemoryStore(),
		LookupEnvironment: environment(map[string]string{
			"AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT": "100",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	description, err := locked.Describe(KeyRateLimitLoginIPLimit)
	if err != nil {
		t.Fatal(err)
	}
	if locked.CurrentRateLimits().LoginIPLimit != 100 || !description.Locked || description.EffectiveSource != SourceEnvironment {
		t.Fatalf("environment-locked rate-limit setting = %+v, policy = %+v", description, locked.CurrentRateLimits())
	}
	if _, err := locked.Set(context.Background(), KeyRateLimitLoginIPLimit, "10", actor, 0); !errors.Is(err, ErrSettingLocked) {
		t.Fatalf("environment-locked rate-limit mutation error = %v", err)
	}
}

func assertEnvironmentDescription(t *testing.T, description Description, source Source, value string, locked bool) {
	t.Helper()
	if description.EffectiveSource != source || description.EffectiveValue != value || description.Locked != locked {
		t.Fatalf("environment description = %+v, want source=%q value=%q locked=%t", description, source, value, locked)
	}
}

func TestCrossFieldValidationUsesTheFullyResolvedHybridPolicy(t *testing.T) {
	store := newMemoryStore()
	store.seed(KeyChallengeVerificationCodeTTL, `"5m"`, 1)
	service, err := NewService(context.Background(), ServiceOptions{Startup: &Config{},
		Store: store,
		LookupEnvironment: environment(map[string]string{
			"AUTHARA_CHALLENGE_TTL": "5m",
		}),
	})
	if err != nil {
		t.Fatalf("New with valid environment/database combination: %v", err)
	}
	if got := service.Current(); got.TTL != 5*time.Minute || got.VerificationCodeTTL != 5*time.Minute {
		t.Fatalf("resolved policy = %+v", got)
	}
}

func TestSetClearValidationAndOptimisticConcurrency(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	service, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatal(err)
	}
	actor := uuid.New()

	set, err := service.Set(ctx, KeyChallengeMaxAttempts, "7", actor, 0)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if service.Current().MaxAttempts != 7 || set.EffectiveSource != SourceOperator || set.Revision != 1 {
		t.Fatalf("set result = %+v, policy = %+v", set, service.Current())
	}
	if store.auditCount() != 1 {
		t.Fatalf("audit count = %d, want 1", store.auditCount())
	}

	if _, err := service.Set(ctx, KeyChallengeMaxAttempts, "8", actor, 0); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale Set error = %v", err)
	}
	if service.Current().MaxAttempts != 7 {
		t.Fatal("stale write changed local snapshot")
	}
	if _, err := service.Set(ctx, KeyChallengeMaxAttempts, "100", actor, set.Revision); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("unsafe bound error = %v", err)
	}
	if store.auditCount() != 1 {
		t.Fatal("invalid update was persisted or audited")
	}
	if _, err := service.Set(ctx, KeyChallengeVerificationCodeTTL, "1h", actor, 0); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("cross-setting validation error = %v", err)
	}
	if _, err := service.Set(ctx, KeyChallengeEnabled, "true", actor, 0); !errors.Is(err, ErrSettingLocked) {
		t.Fatalf("environment-only Set error = %v", err)
	}
	if _, err := service.Set(ctx, Key("database.password"), "secret", actor, 0); !errors.Is(err, ErrUnknownSetting) {
		t.Fatalf("unknown Set error = %v", err)
	}

	cleared, err := service.Clear(ctx, KeyChallengeMaxAttempts, actor, set.Revision)
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if service.Current().MaxAttempts != 5 || cleared.EffectiveSource != SourceDefault || cleared.Revision != 0 {
		t.Fatalf("clear result = %+v, policy = %+v", cleared, service.Current())
	}
	if store.auditCount() != 2 {
		t.Fatalf("audit count = %d, want 2", store.auditCount())
	}
	if _, err := service.Clear(ctx, KeyChallengeMaxAttempts, actor, set.Revision); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("second Clear error = %v", err)
	}
}

func TestInvalidPersistedSettingsFailStartupWithoutPublishing(t *testing.T) {
	tests := []struct {
		name string
		key  Key
		raw  string
	}{
		{name: "unknown", key: "unknown.key", raw: `true`},
		{name: "wrong JSON type", key: KeyChallengeMaxAttempts, raw: `"five"`},
		{name: "outside bounds", key: KeyChallengeMaxAttempts, raw: `100`},
		{name: "environment only", key: KeyChallengeEnabled, raw: `true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryStore()
			store.seed(tt.key, tt.raw, 1)
			if _, err := NewService(context.Background(), ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)}); err == nil {
				t.Fatal("New succeeded with invalid persisted setting")
			}
		})
	}
}

func TestTypedParsersKeepDomainTypesAndValidation(t *testing.T) {
	min := 1
	max := 10
	tests := []struct {
		name       string
		definition Definition
		raw        string
		want       any
	}{
		{name: "bool", definition: Definition{Key: "bool", Type: TypeBool}, raw: "true", want: true},
		{name: "int", definition: Definition{Key: "int", Type: TypeInt, minInt: &min, maxInt: &max}, raw: "7", want: 7},
		{name: "duration", definition: Definition{Key: "duration", Type: TypeDuration}, raw: "90s", want: 90 * time.Second},
		{name: "string", definition: Definition{Key: "string", Type: TypeString}, raw: "value", want: "value"},
		{name: "enum", definition: Definition{Key: "enum", Type: TypeEnum, Allowed: []string{"safe", "strict"}}, raw: "strict", want: "strict"},
		{name: "url", definition: Definition{Key: "url", Type: TypeURL}, raw: "https://authara.example", want: "https://authara.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseText(tt.definition, tt.raw, true)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("parsed value = %#v (%T), want %#v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
	if _, err := parseText(Definition{Key: "enum", Type: TypeEnum, Allowed: []string{"safe"}}, "unsafe", true); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("invalid enum error = %v", err)
	}
	if _, err := parseText(Definition{Key: "url", Type: TypeURL}, "/relative", true); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("invalid URL error = %v", err)
	}
}

func TestReconciliationRepairsAnotherReplicaAndPersistsAcrossReconstruction(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	first, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil), ReconcileInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := first.Set(ctx, KeyChallengeTTL, "2h", uuid.New(), 0); err != nil {
		t.Fatal(err)
	}
	if first.Current().TTL != 2*time.Hour {
		t.Fatal("local process did not publish before Set returned")
	}
	if second.Current().TTL != 30*time.Minute {
		t.Fatal("second replica changed without reconciliation")
	}
	reconcileCtx, cancel := context.WithCancel(ctx)
	second.StartReconciler(reconcileCtx)
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for second.Current().TTL != 2*time.Hour {
		select {
		case <-deadline.C:
			cancel()
			t.Fatal("periodic reconciliation did not repair the second replica")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()

	restarted, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Current().TTL != 2*time.Hour {
		t.Fatal("persisted override did not survive service reconstruction")
	}
}

func TestMutationRefreshesAndValidatesAgainstTheLatestReplicaState(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	first, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatal(err)
	}
	actor := uuid.New()

	if _, err := first.Set(ctx, KeyChallengeTTL, "12m", actor, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Set(ctx, KeyChallengeVerificationCodeTTL, "13m", actor, 0); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("validation against another replica's change = %v", err)
	}
	if _, err := second.Set(ctx, KeyChallengeMaxAttempts, "8", actor, 0); err != nil {
		t.Fatal(err)
	}
	if got := second.Current(); got.TTL != 12*time.Minute || got.MaxAttempts != 8 {
		t.Fatalf("second replica published an incomplete snapshot: %+v", got)
	}
	state, err := store.LoadRuntimeSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Overrides) != 2 {
		t.Fatalf("stored overrides = %+v", state.Overrides)
	}
}

func TestMutationPublishesWithoutAPostCommitReload(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		store := &postMutationLoadFailStore{memoryStore: newMemoryStore()}
		service, err := NewService(context.Background(), ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
		if err != nil {
			t.Fatal(err)
		}

		description, err := service.Set(context.Background(), KeyChallengeMaxAttempts, "8", uuid.New(), 0)
		if err != nil {
			t.Fatalf("Set returned an error after persistence: %v", err)
		}
		snap := service.current.Load()
		if store.loads != 2 {
			t.Fatalf("loads = %d, want initial and pre-commit loads only", store.loads)
		}
		if snap.revision != description.Revision || snap.challenge.MaxAttempts != 8 {
			t.Fatalf("snapshot = %+v, description = %+v", snap, description)
		}
		persisted, ok := snap.overrides[KeyChallengeMaxAttempts]
		if !ok || persisted.Revision != description.Revision || persisted.UpdatedByUserID == nil {
			t.Fatalf("published override = %+v, description = %+v", persisted, description)
		}
	})

	t.Run("clear", func(t *testing.T) {
		base := newMemoryStore()
		base.seed(KeyChallengeMaxAttempts, `7`, 4)
		store := &postMutationLoadFailStore{memoryStore: base}
		service, err := NewService(context.Background(), ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
		if err != nil {
			t.Fatal(err)
		}

		description, err := service.Clear(context.Background(), KeyChallengeMaxAttempts, uuid.New(), 4)
		if err != nil {
			t.Fatalf("Clear returned an error after persistence: %v", err)
		}
		snap := service.current.Load()
		if store.loads != 2 {
			t.Fatalf("loads = %d, want initial and pre-commit loads only", store.loads)
		}
		if snap.revision != 5 || snap.challenge.MaxAttempts != 5 || description.EffectiveSource != SourceDefault {
			t.Fatalf("snapshot = %+v, description = %+v", snap, description)
		}
		if _, ok := snap.overrides[KeyChallengeMaxAttempts]; ok {
			t.Fatalf("cleared override remained in published snapshot: %+v", snap.overrides)
		}
	})
}

func TestMutationRefreshesSnapshotAfterAStoreConflict(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		store := &conflictOnMutationStore{
			memoryStore: newMemoryStore(),
			key:         KeyChallengeMaxAttempts,
			value:       json.RawMessage(`9`),
		}
		service, err := NewService(context.Background(), ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := service.Set(context.Background(), KeyChallengeMaxAttempts, "8", uuid.New(), 0); !errors.Is(err, ErrRevisionConflict) {
			t.Fatalf("Set error = %v, want revision conflict", err)
		}
		description, err := service.Describe(KeyChallengeMaxAttempts)
		if err != nil {
			t.Fatal(err)
		}
		if service.Current().MaxAttempts != 9 || description.Revision != 1 {
			t.Fatalf("conflict snapshot was not refreshed: policy=%+v description=%+v", service.Current(), description)
		}
	})

	t.Run("clear", func(t *testing.T) {
		base := newMemoryStore()
		base.seed(KeyChallengeMaxAttempts, `7`, 1)
		store := &conflictOnMutationStore{
			memoryStore: base,
			key:         KeyChallengeMaxAttempts,
			value:       json.RawMessage(`9`),
		}
		service, err := NewService(context.Background(), ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := service.Clear(context.Background(), KeyChallengeMaxAttempts, uuid.New(), 1); !errors.Is(err, ErrRevisionConflict) {
			t.Fatalf("Clear error = %v, want revision conflict", err)
		}
		description, err := service.Describe(KeyChallengeMaxAttempts)
		if err != nil {
			t.Fatal(err)
		}
		if service.Current().MaxAttempts != 9 || description.Revision != 2 {
			t.Fatalf("conflict snapshot was not refreshed: policy=%+v description=%+v", service.Current(), description)
		}
	})
}

func TestConcurrentReadsObserveOnlyWholeSnapshots(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	service, err := NewService(ctx, ServiceOptions{Startup: &Config{}, Store: store, LookupEnvironment: environment(nil)})
	if err != nil {
		t.Fatal(err)
	}
	actor := uuid.New()

	var wg sync.WaitGroup
	errCh := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 2000 {
				policy := service.Current()
				if policy.TTL != 30*time.Minute || policy.VerificationCodeTTL != 10*time.Minute {
					errCh <- errors.New("reader observed a partial or unexpected snapshot")
					return
				}
			}
		}()
	}
	for i := range 100 {
		description, _ := service.Describe(KeyChallengeMaxAttempts)
		value := "6"
		if i%2 == 1 {
			value = "7"
		}
		if _, err := service.Set(ctx, KeyChallengeMaxAttempts, value, actor, description.Revision); err != nil {
			t.Fatalf("Set iteration %d: %v", i, err)
		}
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func environment(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

type memoryStore struct {
	mu        sync.Mutex
	revision  int64
	overrides map[Key]PersistedOverride
	audits    []json.RawMessage
}

type postMutationLoadFailStore struct {
	*memoryStore
	loads             int
	mutationSucceeded bool
}

type conflictOnMutationStore struct {
	*memoryStore
	key   Key
	value json.RawMessage
}

func (s *conflictOnMutationStore) UpsertRuntimeSettingOverride(context.Context, Mutation) (PersistedOverride, error) {
	s.simulateCompetingMutation()
	return PersistedOverride{}, ErrRevisionConflict
}

func (s *conflictOnMutationStore) DeleteRuntimeSettingOverride(context.Context, Key, int64, int64, uuid.UUID, json.RawMessage) (int64, error) {
	s.simulateCompetingMutation()
	return 0, ErrRevisionConflict
}

func (s *conflictOnMutationStore) simulateCompetingMutation() {
	s.memoryStore.mu.Lock()
	defer s.memoryStore.mu.Unlock()
	s.memoryStore.revision++
	s.memoryStore.overrides[s.key] = PersistedOverride{
		Key: s.key, Value: append(json.RawMessage(nil), s.value...), Revision: s.memoryStore.revision,
	}
}

func (s *postMutationLoadFailStore) LoadRuntimeSettings(ctx context.Context) (PersistedState, error) {
	s.loads++
	if s.mutationSucceeded {
		return PersistedState{}, errors.New("post-commit load must not be attempted")
	}
	return s.memoryStore.LoadRuntimeSettings(ctx)
}

func (s *postMutationLoadFailStore) UpsertRuntimeSettingOverride(ctx context.Context, mutation Mutation) (PersistedOverride, error) {
	saved, err := s.memoryStore.UpsertRuntimeSettingOverride(ctx, mutation)
	if err == nil {
		s.mutationSucceeded = true
	}
	return saved, err
}

func (s *postMutationLoadFailStore) DeleteRuntimeSettingOverride(
	ctx context.Context,
	key Key,
	expectedRevision int64,
	expectedStateRevision int64,
	actorID uuid.UUID,
	metadata json.RawMessage,
) (int64, error) {
	revision, err := s.memoryStore.DeleteRuntimeSettingOverride(
		ctx, key, expectedRevision, expectedStateRevision, actorID, metadata,
	)
	if err == nil {
		s.mutationSucceeded = true
	}
	return revision, err
}

func newMemoryStore() *memoryStore {
	return &memoryStore{overrides: make(map[Key]PersistedOverride)}
}

func (s *memoryStore) seed(key Key, raw string, revision int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overrides[key] = PersistedOverride{Key: key, Value: json.RawMessage(raw), Revision: revision}
	if revision > s.revision {
		s.revision = revision
	}
}

func (s *memoryStore) LoadRuntimeSettings(context.Context) (PersistedState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return PersistedState{Revision: s.revision, Overrides: overrideSlice(cloneOverrides(s.overrides))}, nil
}

func (s *memoryStore) UpsertRuntimeSettingOverride(_ context.Context, mutation Mutation) (PersistedOverride, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mutation.ExpectedStateRevision != s.revision {
		return PersistedOverride{}, ErrRevisionConflict
	}
	current, exists := s.overrides[mutation.Key]
	if (!exists && mutation.ExpectedRevision != 0) || (exists && current.Revision != mutation.ExpectedRevision) {
		return PersistedOverride{}, ErrRevisionConflict
	}
	s.revision++
	now := time.Now().UTC()
	saved := PersistedOverride{
		Key: mutation.Key, Value: append(json.RawMessage(nil), mutation.Value...), Revision: s.revision,
		CreatedAt: current.CreatedAt, UpdatedAt: now, UpdatedByUserID: &mutation.ActorUserID,
	}
	if saved.CreatedAt.IsZero() {
		saved.CreatedAt = now
	}
	s.overrides[mutation.Key] = saved
	s.audits = append(s.audits, append(json.RawMessage(nil), mutation.AuditMetadata...))
	return cloneOverride(saved), nil
}

func (s *memoryStore) DeleteRuntimeSettingOverride(_ context.Context, key Key, expected, expectedState int64, _ uuid.UUID, metadata json.RawMessage) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if expectedState != s.revision {
		return 0, ErrRevisionConflict
	}
	current, exists := s.overrides[key]
	if !exists || current.Revision != expected {
		return 0, ErrRevisionConflict
	}
	delete(s.overrides, key)
	s.revision++
	s.audits = append(s.audits, append(json.RawMessage(nil), metadata...))
	return s.revision, nil
}

func (s *memoryStore) auditCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.audits)
}
