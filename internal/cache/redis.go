package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type Redis struct {
	client *redis.Client
}

func NewRedis(cfg RedisConfig) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			return nil, fmt.Errorf("ping redis: %w; close redis: %w", err, closeErr)
		}
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Redis{client: client}, nil
}

func (r *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	if err != nil {
		return nil, fmt.Errorf("get %q: %w", key, err)
	}
	return value, nil
}

func (r *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := r.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("set %q: %w", key, err)
	}
	return nil
}

func (r *Redis) Delete(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

func (r *Redis) Increment(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (count int64, remainingTTL time.Duration, err error) {
	result, err := incrementScript.Run(ctx, r.client, []string{key}, ttl.Milliseconds()).Slice()
	if err != nil {
		return 0, 0, fmt.Errorf("increment %q: %w", key, err)
	}
	if len(result) != 2 {
		return 0, 0, fmt.Errorf("increment %q: unexpected redis result", key)
	}

	count, ok := result[0].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("increment %q: unexpected count type %T", key, result[0])
	}

	ttlMillis, ok := result[1].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("increment %q: unexpected ttl type %T", key, result[1])
	}
	if ttlMillis < 0 {
		ttlMillis = 0
	}

	return count, time.Duration(ttlMillis) * time.Millisecond, nil
}

func (r *Redis) Close() error {
	return r.client.Close()
}

var incrementScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {count, ttl}
`)
