package httpctx

import (
	"context"

	"github.com/google/uuid"
)

type sessionIDKeyType struct{}

var sessionIDKey sessionIDKeyType

func WithSessionID(ctx context.Context, sessionID uuid.UUID) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

func SessionID(ctx context.Context) (uuid.UUID, bool) {
	id, err := ctx.Value(sessionIDKey).(uuid.UUID)
	return id, err
}
