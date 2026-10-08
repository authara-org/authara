package cache

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRedisSetMaxInt64IsMonotonic(t *testing.T) {
	host := os.Getenv("AUTHARA_TEST_REDIS_HOST")
	if host == "" {
		t.Skip("AUTHARA_TEST_REDIS_HOST is not set")
	}
	port := 6379
	if rawPort := os.Getenv("AUTHARA_TEST_REDIS_PORT"); rawPort != "" {
		parsed, err := strconv.Atoi(rawPort)
		if err != nil {
			t.Fatalf("parse AUTHARA_TEST_REDIS_PORT: %v", err)
		}
		port = parsed
	}

	redisCache, err := NewRedis(RedisConfig{Host: host, Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = redisCache.Close() }()
	key := "authara:test:set-max:" + uuid.NewString()
	defer func() { _ = redisCache.Delete(context.Background(), key) }()

	ctx := context.Background()
	if err := redisCache.SetMaxInt64(ctx, key, 200, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redisCache.SetMaxInt64(ctx, key, 100, 2*time.Minute); err != nil {
		t.Fatal(err)
	}
	value, err := redisCache.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "200" {
		t.Fatalf("value after older delayed write = %q, want 200", value)
	}

	if err := redisCache.SetMaxInt64(ctx, key, 300, time.Minute); err != nil {
		t.Fatal(err)
	}
	value, err = redisCache.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "300" {
		t.Fatalf("value after newer write = %q, want 300", value)
	}
	remaining, err := redisCache.client.PTTL(ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	if remaining < 90*time.Second {
		t.Fatalf("TTL after newer write = %s, want the previous longer TTL to be preserved", remaining)
	}
}
