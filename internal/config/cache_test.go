package config

import (
	"strings"
	"testing"
)

func TestCacheValidateAllowsNoop(t *testing.T) {
	cfg := Cache{Provider: "noop", RedisHost: "localhost", RedisPort: 6379}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestCacheValidateAllowsRedis(t *testing.T) {
	cfg := Cache{Provider: "redis", RedisHost: "localhost", RedisPort: 6379}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestCacheValidateRejectsInvalidProvider(t *testing.T) {
	cfg := Cache{Provider: "memcached", RedisHost: "localhost", RedisPort: 6379}

	if err := cfg.validate(); err == nil {
		t.Fatal("expected invalid provider error")
	}
}

func TestCacheValidateRejectsRedisWithoutHost(t *testing.T) {
	cfg := Cache{Provider: "redis", RedisPort: 6379}

	if err := cfg.validate(); err == nil {
		t.Fatal("expected missing host error")
	}
}

func TestCacheValidateNormalizesAccessTokenRevocationMode(t *testing.T) {
	cfg := Cache{
		Provider:                  " REDIS ",
		AccessTokenRevocationMode: " IMMEDIATE ",
		RedisHost:                 "localhost",
		RedisPort:                 6379,
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if cfg.Provider != "redis" || cfg.AccessTokenRevocationMode != AccessTokenRevocationModeImmediate {
		t.Fatalf("normalized cache config = %+v", cfg)
	}
}

func TestCacheValidateRejectsInvalidAccessTokenRevocationMode(t *testing.T) {
	cfg := Cache{
		Provider:                  "noop",
		AccessTokenRevocationMode: "best-effort",
		RedisHost:                 "localhost",
		RedisPort:                 6379,
	}

	err := cfg.validate()
	if err == nil || !strings.Contains(err.Error(), "AUTHARA_ACCESS_TOKEN_REVOCATION_MODE") {
		t.Fatalf("validate error = %v", err)
	}
}
