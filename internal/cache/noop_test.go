package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNoopGetReturnsMiss(t *testing.T) {
	c := NewNoop()

	_, err := c.Get(context.Background(), "missing")
	if !errors.Is(err, ErrMiss) {
		t.Fatalf("expected ErrMiss, got %v", err)
	}
}

func TestNoopMutationsSucceed(t *testing.T) {
	c := NewNoop()

	if err := c.Set(context.Background(), "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if err := c.Delete(context.Background(), "key"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}
