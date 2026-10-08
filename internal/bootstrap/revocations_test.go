package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestNewAccessTokenRevocationsHonorsSelectedMode(t *testing.T) {
	for _, tt := range []struct {
		mode       string
		wantWrites int
	}{
		{mode: config.AccessTokenRevocationModeImmediate, wantWrites: 1},
		{mode: config.AccessTokenRevocationModeExpiry, wantWrites: 0},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			backend := &revocationBackend{}
			revocations := NewAccessTokenRevocations(
				&config.Config{Cache: config.Cache{AccessTokenRevocationMode: tt.mode}},
				backend,
				func() time.Duration { return time.Hour },
			)

			if err := revocations.RevokeUser(context.Background(), uuid.New(), time.Now()); err != nil {
				t.Fatal(err)
			}
			if backend.writes != tt.wantWrites {
				t.Fatalf("writes = %d, want %d", backend.writes, tt.wantWrites)
			}
		})
	}
}

func TestLogAccessTokenRevocationGuarantee(t *testing.T) {
	for _, tt := range []struct {
		mode string
		want string
	}{
		{mode: config.AccessTokenRevocationModeImmediate, want: "deny protected requests"},
		{mode: config.AccessTokenRevocationModeExpiry, want: "issued access tokens may remain valid"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, nil))
			logAccessTokenRevocationGuarantee(&config.Config{
				Cache: config.Cache{Provider: "noop", AccessTokenRevocationMode: tt.mode},
				Token: config.Token{AccessTokenTTL: config.ExpiryOnlyMaxAccessTokenTTL},
			}, logger)

			if logged := output.String(); !strings.Contains(logged, "mode="+tt.mode) || !strings.Contains(logged, tt.want) {
				t.Fatalf("log = %q", logged)
			}
		})
	}
}

func TestAccessTokenRevocationModesWhenBackendIsUnavailable(t *testing.T) {
	storeFailure := errors.New("redis unavailable")
	backend := &revocationBackend{readErr: storeFailure}
	claims := &token.AccessClaims{
		SessionID: uuid.New(),
		OrgID:     uuid.New(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  uuid.NewString(),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	immediate := NewAccessTokenRevocations(
		&config.Config{Cache: config.Cache{AccessTokenRevocationMode: config.AccessTokenRevocationModeImmediate}},
		backend,
		func() time.Duration { return time.Hour },
	)
	if err := immediate.Check(context.Background(), "token", claims); !errors.Is(err, token.ErrRevocationStoreUnavailable) {
		t.Fatalf("immediate Check error = %v, want ErrRevocationStoreUnavailable", err)
	}

	expiry := NewAccessTokenRevocations(
		&config.Config{Cache: config.Cache{AccessTokenRevocationMode: config.AccessTokenRevocationModeExpiry}},
		backend,
		func() time.Duration { return time.Hour },
	)
	if err := expiry.Check(context.Background(), "token", claims); err != nil {
		t.Fatalf("expiry Check error = %v, want no online check", err)
	}
}

func TestAccessTokenRevocationMarkerTTLCoversDynamicMaximum(t *testing.T) {
	if got := AccessTokenRevocationMarkerTTL(10 * time.Minute); got != 24*time.Hour {
		t.Fatalf("short access-token marker TTL = %s", got)
	}
	if got := AccessTokenRevocationMarkerTTL(48 * time.Hour); got != 48*time.Hour {
		t.Fatalf("long access-token marker TTL = %s", got)
	}
}

type revocationBackend struct {
	writes  int
	readErr error
}

func (*revocationBackend) Get(context.Context, string) ([]byte, error) { return nil, nil }

func (b *revocationBackend) GetMany(_ context.Context, keys ...string) ([][]byte, error) {
	if b.readErr != nil {
		return nil, b.readErr
	}
	return make([][]byte, len(keys)), nil
}

func (b *revocationBackend) Set(context.Context, string, []byte, time.Duration) error {
	b.writes++
	return nil
}

func (b *revocationBackend) SetMaxInt64(context.Context, string, int64, time.Duration) error {
	b.writes++
	return nil
}

func (*revocationBackend) Delete(context.Context, string) error { return nil }
func (*revocationBackend) Close() error                         { return nil }
