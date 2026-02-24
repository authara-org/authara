package httpctx

import (
	"context"

	"github.com/google/uuid"
)

type userIDKeyType struct{}

var userIDKey = userIDKeyType{}

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}
