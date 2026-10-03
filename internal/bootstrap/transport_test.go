package bootstrap

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/config"
)

func TestWarnIfTransportMayBeInsecureWarnsWithoutRejectingConfiguration(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	cfg := &config.Config{
		Values: config.Values{AppEnv: "prod", PublicURL: "http://auth.example.com"},
		Cache:  config.Cache{Provider: "redis", RedisHost: "redis.internal", RedisPort: 6379},
	}

	warnIfTransportMayBeInsecure(cfg, logger)

	logs := output.String()
	for _, want := range []string{"PUBLIC_URL does not use HTTPS", "Redis transport is not encrypted"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs = %q, want warning containing %q", logs, want)
		}
	}
}

func TestWarnIfTransportMayBeInsecureDoesNotWarnForHTTPSWithoutRedis(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	cfg := &config.Config{
		Values: config.Values{AppEnv: "prod", PublicURL: "https://auth.example.com"},
		Cache:  config.Cache{Provider: "noop"},
	}

	warnIfTransportMayBeInsecure(cfg, logger)

	if output.Len() != 0 {
		t.Fatalf("unexpected warning logs: %s", output.String())
	}
}
