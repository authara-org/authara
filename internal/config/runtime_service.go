package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const DefaultReconcileInterval = 2 * time.Second

type ServiceOptions struct {
	Startup           *Config
	Store             RuntimeSettingsStore
	LookupEnvironment func(string) (string, bool)
	Environment       []EnvironmentVariable
	Logger            *slog.Logger
	ReconcileInterval time.Duration
}

type snapshot struct {
	revision     int64
	challenge    ChallengePolicy
	rateLimits   RateLimitPolicy
	descriptions map[Key]Description
	overrides    map[Key]PersistedOverride
}

type Service struct {
	*Config
	store               RuntimeSettingsStore
	definitions         []Definition
	definitionByKey     map[Key]Definition
	defaultValues       map[Key]any
	environmentValues   map[Key]any
	explicitEnvironment map[Key]bool
	logger              *slog.Logger
	interval            time.Duration
	mu                  sync.Mutex
	current             atomic.Pointer[snapshot]
}

func NewService(ctx context.Context, cfg ServiceOptions) (*Service, error) {
	if cfg.Startup == nil {
		return nil, errors.New("startup config is required")
	}
	if cfg.Store == nil {
		return nil, errors.New("runtime settings store is required")
	}
	lookup := cfg.LookupEnvironment
	if lookup == nil {
		lookup = os.LookupEnv
	}
	interval := cfg.ReconcileInterval
	if interval <= 0 {
		interval = DefaultReconcileInterval
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	s := &Service{
		Config: cfg.Startup,
		store:  cfg.Store, logger: logger, interval: interval,
		definitions: buildCatalog(cfg.Environment), definitionByKey: make(map[Key]Definition),
		defaultValues: make(map[Key]any), environmentValues: make(map[Key]any), explicitEnvironment: make(map[Key]bool),
	}
	for _, definition := range s.definitions {
		if _, exists := s.definitionByKey[definition.Key]; exists {
			return nil, fmt.Errorf("duplicate runtime setting key %q", definition.Key)
		}
		s.definitionByKey[definition.Key] = definition
		s.defaultValues[definition.Key] = definition.defaultValue
		if definition.Environment == "" {
			continue
		}
		raw, explicit := lookup(definition.Environment)
		if !explicit {
			continue
		}
		if raw == "" && !definition.Required && (!definition.HasDefault || definition.Environment == "LOG_LEVEL") {
			// Optional empty values represent an unconfigured setting. LOG_LEVEL
			// also resolves an explicitly empty value to its environment-specific
			// default in config.Load.
			continue
		}
		s.explicitEnvironment[definition.Key] = true
		if definition.Sensitive {
			// Keep only presence information for secrets. The config package owns
			// the actual value; runtime settings must never retain or render it.
			s.environmentValues[definition.Key] = ""
			continue
		}
		value := any(raw)
		if isRuntimePolicyKey(definition.Key) {
			var err error
			value, err = parseText(definition, raw, false)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", definition.Environment, err)
			}
		}
		s.environmentValues[definition.Key] = value
	}
	if !s.explicitEnvironment[Key("LOG_LEVEL")] {
		if appEnvironment, ok := s.environmentValues[Key("APP_ENV")].(string); ok && strings.EqualFold(strings.TrimSpace(appEnvironment), "prod") {
			s.defaultValues[Key("LOG_LEVEL")] = "info"
		}
	}
	if err := s.reloadLocked(ctx, true); err != nil {
		return nil, fmt.Errorf("load runtime settings: %w", err)
	}
	return s, nil
}

// Startup returns the immutable environment configuration used to construct
// process-level dependencies. Dynamic consumers should use the typed policy
// readers exposed by Service instead.
func (s *Service) Startup() *Config {
	return s.Config
}

func (s *Service) Current() ChallengePolicy {
	return s.current.Load().challenge
}

func (s *Service) CurrentRateLimits() RateLimitPolicy {
	return s.current.Load().rateLimits
}

func (s *Service) Describe(key Key) (Description, error) {
	description, ok := s.current.Load().descriptions[key]
	if !ok {
		return Description{}, fmt.Errorf("%w: %q", ErrUnknownSetting, key)
	}
	return cloneDescription(description), nil
}

func (s *Service) List() []Description {
	snap := s.current.Load()
	out := make([]Description, 0, len(s.definitions))
	for _, definition := range s.definitions {
		description := snap.descriptions[definition.Key]
		out = append(out, cloneDescription(description))
	}
	return out
}

func (s *Service) Set(ctx context.Context, key Key, rawValue string, actorID uuid.UUID, expectedRevision int64) (Description, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if actorID == uuid.Nil {
		return Description{}, ErrMissingActor
	}
	if expectedRevision < 0 {
		return Description{}, fmt.Errorf("%w: revision must not be negative", ErrInvalidValue)
	}
	definition, err := s.mutableDefinition(key)
	if err != nil {
		return Description{}, err
	}
	value, err := parseText(definition, rawValue, true)
	if err != nil {
		return Description{}, err
	}
	encoded, err := encodeValue(definition, value)
	if err != nil {
		return Description{}, fmt.Errorf("encode runtime setting %q: %w", key, err)
	}
	if err := s.reloadLocked(ctx, true); err != nil {
		return Description{}, fmt.Errorf("reload runtime settings before saving %q: %w", key, err)
	}

	before := s.current.Load()
	overrides := cloneOverrides(before.overrides)
	previewRevision := before.revision + 1
	if previewRevision <= 0 {
		previewRevision = 1
	}
	overrides[key] = PersistedOverride{Key: key, Value: encoded, Revision: previewRevision}
	proposed, err := s.buildSnapshot(PersistedState{Revision: before.revision, Overrides: overrideSlice(overrides)})
	if err != nil {
		return Description{}, err
	}
	metadata := mutationAuditMetadata(before.descriptions[key], proposed.descriptions[key])
	saved, err := s.store.UpsertRuntimeSettingOverride(ctx, Mutation{
		Key: key, Value: encoded, ExpectedRevision: expectedRevision, ExpectedStateRevision: before.revision,
		ActorUserID: actorID, AuditMetadata: metadata,
	})
	if err != nil {
		s.refreshAfterConflict(ctx, key, err)
		return Description{}, err
	}
	proposed.revision = saved.Revision
	proposed.overrides[key] = cloneOverride(saved)
	description := proposed.descriptions[key]
	description.Revision = saved.Revision
	proposed.descriptions[key] = description
	s.current.Store(proposed)
	return cloneDescription(description), nil
}

func encodeValue(definition Definition, value any) (json.RawMessage, error) {
	if definition.Type == TypeDuration {
		value = value.(time.Duration).String()
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func (s *Service) Clear(ctx context.Context, key Key, actorID uuid.UUID, expectedRevision int64) (Description, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if actorID == uuid.Nil {
		return Description{}, ErrMissingActor
	}
	if expectedRevision <= 0 {
		return Description{}, fmt.Errorf("%w: an active override revision is required", ErrRevisionConflict)
	}
	if _, err := s.mutableDefinition(key); err != nil {
		return Description{}, err
	}
	if err := s.reloadLocked(ctx, true); err != nil {
		return Description{}, fmt.Errorf("reload runtime settings before clearing %q: %w", key, err)
	}
	before := s.current.Load()
	overrides := cloneOverrides(before.overrides)
	if current, ok := overrides[key]; !ok || current.Revision != expectedRevision {
		return Description{}, ErrRevisionConflict
	}
	delete(overrides, key)
	proposed, err := s.buildSnapshot(PersistedState{Revision: before.revision, Overrides: overrideSlice(overrides)})
	if err != nil {
		return Description{}, err
	}
	metadata := mutationAuditMetadata(before.descriptions[key], proposed.descriptions[key])
	revision, err := s.store.DeleteRuntimeSettingOverride(ctx, key, expectedRevision, before.revision, actorID, metadata)
	if err != nil {
		s.refreshAfterConflict(ctx, key, err)
		return Description{}, err
	}
	proposed.revision = revision
	s.current.Store(proposed)
	return cloneDescription(proposed.descriptions[key]), nil
}

func (s *Service) refreshAfterConflict(ctx context.Context, key Key, mutationErr error) {
	if !errors.Is(mutationErr, ErrRevisionConflict) {
		return
	}
	if err := s.reloadLocked(ctx, true); err != nil {
		s.logger.WarnContext(ctx, "refresh runtime settings after revision conflict failed", "key", key, "err", err)
	}
}

func (s *Service) mutableDefinition(key Key) (Definition, error) {
	definition, ok := s.definitionByKey[key]
	if !ok {
		return Definition{}, fmt.Errorf("%w: %q", ErrUnknownSetting, key)
	}
	if definition.Sensitive || definition.Reload != ReloadDynamic || definition.Control == ControlEnvironment {
		return Definition{}, fmt.Errorf("%w: %q is managed at startup", ErrSettingLocked, key)
	}
	if definition.Control == ControlHybrid {
		if s.explicitEnvironment[key] {
			return Definition{}, fmt.Errorf("%w: %q is managed by %s", ErrSettingLocked, key, definition.Environment)
		}
	}
	return definition, nil
}

func (s *Service) StartReconciler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Reconcile(ctx); err != nil && ctx.Err() == nil {
					s.logger.ErrorContext(ctx, "runtime settings reconciliation failed", "err", err)
				}
			}
		}
	}()
}

func (s *Service) Reconcile(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadLocked(ctx, false)
}

func (s *Service) reloadLocked(ctx context.Context, force bool) error {
	state, err := s.store.LoadRuntimeSettings(ctx)
	if err != nil {
		return err
	}
	if current := s.current.Load(); !force && current != nil && current.revision == state.Revision {
		return nil
	}
	next, err := s.buildSnapshot(state)
	if err != nil {
		return err
	}
	s.current.Store(next)
	return nil
}

func (s *Service) buildSnapshot(state PersistedState) (*snapshot, error) {
	overrides := make(map[Key]PersistedOverride, len(state.Overrides))
	for _, override := range state.Overrides {
		definition, ok := s.definitionByKey[override.Key]
		if !ok {
			return nil, fmt.Errorf("%w in database: %q", ErrUnknownSetting, override.Key)
		}
		if definition.Control == ControlEnvironment || definition.Sensitive || definition.Reload != ReloadDynamic {
			return nil, fmt.Errorf("%w in database: %q is not operator-editable", ErrInvalidValue, override.Key)
		}
		if override.Revision <= 0 {
			return nil, fmt.Errorf("%w in database: %q has revision %d", ErrInvalidValue, override.Key, override.Revision)
		}
		if _, exists := overrides[override.Key]; exists {
			return nil, fmt.Errorf("%w in database: duplicate %q", ErrInvalidValue, override.Key)
		}
		if _, err := parseJSON(definition, override.Value, true); err != nil {
			return nil, fmt.Errorf("stored %s: %w", override.Key, err)
		}
		overrides[override.Key] = cloneOverride(override)
	}

	descriptions := make(map[Key]Description, len(s.definitions))
	values := make(map[Key]any, len(s.definitions))
	for _, definition := range cloneCatalog(s.definitions) {
		value := s.defaultValues[definition.Key]
		source := SourceDefault
		if !definition.HasDefault {
			source = SourceUnset
		}
		locked := definition.Control == ControlEnvironment
		var persisted *string
		var revision int64
		dormant := false
		if override, ok := overrides[definition.Key]; ok {
			parsed, err := parseJSON(definition, override.Value, true)
			if err != nil {
				return nil, fmt.Errorf("stored %s: %w", definition.Key, err)
			}
			value = parsed
			source = SourceOperator
			formatted := formatValue(parsed)
			persisted = &formatted
			revision = override.Revision
		}
		if s.explicitEnvironment[definition.Key] {
			if environmentValue, ok := s.environmentValues[definition.Key]; ok {
				value = environmentValue
				source = SourceEnvironment
				locked = true
				dormant = persisted != nil
			}
		}
		values[definition.Key] = value
		effectiveValue := formatValue(value)
		if definition.Sensitive && source == SourceEnvironment {
			effectiveValue = "Configured (value hidden)"
		}
		descriptions[definition.Key] = Description{
			Definition: definition, EffectiveValue: effectiveValue, EffectiveSource: source,
			Locked: locked, PersistedOverride: persisted, Revision: revision, DormantOverride: dormant,
		}
	}

	policy := ChallengePolicy{
		Enabled:               values[KeyChallengeEnabled].(bool),
		TTL:                   values[KeyChallengeTTL].(time.Duration),
		VerificationCodeTTL:   values[KeyChallengeVerificationCodeTTL].(time.Duration),
		MaxAttempts:           values[KeyChallengeMaxAttempts].(int),
		MaxResends:            values[KeyChallengeMaxResends].(int),
		MinimumResendInterval: values[KeyChallengeMinimumResendInterval].(time.Duration),
	}
	if err := validateChallengePolicy(policy); err != nil {
		return nil, err
	}
	rateLimits := RateLimitPolicy{
		LoginIPLimit:     values[KeyRateLimitLoginIPLimit].(int),
		LoginIPWindow:    values[KeyRateLimitLoginIPWindow].(time.Duration),
		LoginEmailLimit:  values[KeyRateLimitLoginEmailLimit].(int),
		LoginEmailWindow: values[KeyRateLimitLoginEmailWindow].(time.Duration),

		SignupIPLimit:     values[KeyRateLimitSignupIPLimit].(int),
		SignupIPWindow:    values[KeyRateLimitSignupIPWindow].(time.Duration),
		SignupEmailLimit:  values[KeyRateLimitSignupEmailLimit].(int),
		SignupEmailWindow: values[KeyRateLimitSignupEmailWindow].(time.Duration),

		PasswordResetIPLimit:     values[KeyRateLimitPasswordResetIPLimit].(int),
		PasswordResetIPWindow:    values[KeyRateLimitPasswordResetIPWindow].(time.Duration),
		PasswordResetEmailLimit:  values[KeyRateLimitPasswordResetEmailLimit].(int),
		PasswordResetEmailWindow: values[KeyRateLimitPasswordResetEmailWindow].(time.Duration),

		PasskeyLoginIPLimit:  values[KeyRateLimitPasskeyLoginIPLimit].(int),
		PasskeyLoginIPWindow: values[KeyRateLimitPasskeyLoginIPWindow].(time.Duration),

		ChallengeVerifyIPLimit:  values[KeyRateLimitChallengeVerifyIPLimit].(int),
		ChallengeVerifyIPWindow: values[KeyRateLimitChallengeVerifyIPWindow].(time.Duration),

		ChallengeResendIPLimit:  values[KeyRateLimitChallengeResendIPLimit].(int),
		ChallengeResendIPWindow: values[KeyRateLimitChallengeResendIPWindow].(time.Duration),

		CleanupEvery: values[KeyRateLimitCleanupEvery].(int),
		MaxEntries:   values[KeyRateLimitMaxEntries].(int),
	}
	if err := validateRateLimitPolicy(rateLimits); err != nil {
		return nil, err
	}
	return &snapshot{revision: state.Revision, challenge: policy, rateLimits: rateLimits, descriptions: descriptions, overrides: overrides}, nil
}

func validateChallengePolicy(policy ChallengePolicy) error {
	if policy.TTL <= 0 {
		return fmt.Errorf("%w: challenge lifetime must be greater than zero", ErrInvalidValue)
	}
	if policy.VerificationCodeTTL <= 0 {
		return fmt.Errorf("%w: verification-code lifetime must be greater than zero", ErrInvalidValue)
	}
	if policy.VerificationCodeTTL > policy.TTL {
		return fmt.Errorf("%w: verification-code lifetime (%s) must not exceed challenge lifetime (%s)", ErrInvalidValue, policy.VerificationCodeTTL, policy.TTL)
	}
	if policy.MaxAttempts <= 0 {
		return fmt.Errorf("%w: maximum attempts must be greater than zero", ErrInvalidValue)
	}
	if policy.MaxResends < 0 {
		return fmt.Errorf("%w: maximum resends must not be negative", ErrInvalidValue)
	}
	if policy.MinimumResendInterval < 0 {
		return fmt.Errorf("%w: minimum resend interval must not be negative", ErrInvalidValue)
	}
	return nil
}

func validateRateLimitPolicy(policy RateLimitPolicy) error {
	positiveCounts := []int{
		policy.LoginIPLimit, policy.LoginEmailLimit,
		policy.SignupIPLimit, policy.SignupEmailLimit,
		policy.PasswordResetIPLimit, policy.PasswordResetEmailLimit,
		policy.PasskeyLoginIPLimit, policy.ChallengeVerifyIPLimit, policy.ChallengeResendIPLimit,
		policy.CleanupEvery, policy.MaxEntries,
	}
	for _, value := range positiveCounts {
		if value <= 0 {
			return fmt.Errorf("%w: rate-limit counts and safety controls must be greater than zero", ErrInvalidValue)
		}
	}
	positiveWindows := []time.Duration{
		policy.LoginIPWindow, policy.LoginEmailWindow,
		policy.SignupIPWindow, policy.SignupEmailWindow,
		policy.PasswordResetIPWindow, policy.PasswordResetEmailWindow,
		policy.PasskeyLoginIPWindow, policy.ChallengeVerifyIPWindow, policy.ChallengeResendIPWindow,
	}
	for _, value := range positiveWindows {
		if value <= 0 {
			return fmt.Errorf("%w: rate-limit windows must be greater than zero", ErrInvalidValue)
		}
	}
	return nil
}

func parseText(definition Definition, raw string, enforceOperatorBounds bool) (any, error) {
	var value any
	var err error
	switch definition.Type {
	case TypeBool:
		value, err = strconv.ParseBool(strings.TrimSpace(raw))
	case TypeInt:
		value, err = strconv.Atoi(strings.TrimSpace(raw))
	case TypeDuration:
		value, err = time.ParseDuration(strings.TrimSpace(raw))
	case TypeString, TypeCSV, TypeMap:
		value = raw
	case TypeEnum:
		value = strings.TrimSpace(raw)
		if !slices.Contains(definition.Allowed, value.(string)) {
			err = fmt.Errorf("must be one of %s", strings.Join(definition.Allowed, ", "))
		}
	case TypeURL:
		value = strings.TrimSpace(raw)
		var parsed *url.URL
		parsed, err = url.Parse(value.(string))
		if err == nil && (parsed.Scheme == "" || parsed.Host == "") {
			err = fmt.Errorf("must include a scheme and host")
		}
	default:
		err = fmt.Errorf("unsupported type %q", definition.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("%w for %s: %v", ErrInvalidValue, definition.Key, err)
	}
	if enforceOperatorBounds {
		if err := validateBounds(definition, value); err != nil {
			return nil, err
		}
	}
	return value, nil
}

func parseJSON(definition Definition, raw json.RawMessage, enforceOperatorBounds bool) (any, error) {
	var target any
	switch definition.Type {
	case TypeBool:
		target = new(bool)
	case TypeInt:
		target = new(int)
	case TypeDuration, TypeString, TypeEnum, TypeURL, TypeCSV, TypeMap:
		target = new(string)
	default:
		return nil, fmt.Errorf("%w for %s: unsupported type %q", ErrInvalidValue, definition.Key, definition.Type)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return nil, fmt.Errorf("%w for %s: invalid %s JSON", ErrInvalidValue, definition.Key, definition.Type)
	}
	var text string
	switch value := target.(type) {
	case *bool:
		text = strconv.FormatBool(*value)
	case *int:
		text = strconv.Itoa(*value)
	case *string:
		text = *value
	}
	return parseText(definition, text, enforceOperatorBounds)
}

func validateBounds(definition Definition, value any) error {
	switch typed := value.(type) {
	case int:
		if definition.minInt != nil && typed < *definition.minInt {
			return fmt.Errorf("%w for %s: must be at least %d", ErrInvalidValue, definition.Key, *definition.minInt)
		}
		if definition.maxInt != nil && typed > *definition.maxInt {
			return fmt.Errorf("%w for %s: must be at most %d", ErrInvalidValue, definition.Key, *definition.maxInt)
		}
	case time.Duration:
		if definition.minDuration != nil && typed < *definition.minDuration {
			return fmt.Errorf("%w for %s: must be at least %s", ErrInvalidValue, definition.Key, *definition.minDuration)
		}
		if definition.maxDuration != nil && typed > *definition.maxDuration {
			return fmt.Errorf("%w for %s: must be at most %s", ErrInvalidValue, definition.Key, *definition.maxDuration)
		}
	}
	return nil
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case time.Duration:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func mutationAuditMetadata(before, after Description) json.RawMessage {
	metadata, _ := json.Marshal(map[string]any{
		"old_effective_value": before.EffectiveValue,
		"old_source":          before.EffectiveSource,
		"new_effective_value": after.EffectiveValue,
		"new_source":          after.EffectiveSource,
	})
	return metadata
}

func cloneOverrides(in map[Key]PersistedOverride) map[Key]PersistedOverride {
	out := make(map[Key]PersistedOverride, len(in))
	for key, override := range in {
		out[key] = cloneOverride(override)
	}
	return out
}

func cloneOverride(in PersistedOverride) PersistedOverride {
	in.Value = append(json.RawMessage(nil), in.Value...)
	return in
}

func cloneDescription(in Description) Description {
	in.Allowed = append([]string(nil), in.Allowed...)
	if in.PersistedOverride != nil {
		value := *in.PersistedOverride
		in.PersistedOverride = &value
	}
	return in
}

func overrideSlice(in map[Key]PersistedOverride) []PersistedOverride {
	out := make([]PersistedOverride, 0, len(in))
	for _, override := range in {
		out = append(out, override)
	}
	return out
}
