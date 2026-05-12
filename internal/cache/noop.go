package cache

import (
	"context"
	"time"
)

type Noop struct{}

func NewNoop() *Noop {
	return &Noop{}
}

func (n *Noop) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, ErrMiss
}

func (n *Noop) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (n *Noop) Delete(ctx context.Context, key string) error {
	return nil
}

func (n *Noop) Close() error {
	return nil
}
