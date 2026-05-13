package config

import "testing"

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
