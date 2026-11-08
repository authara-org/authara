package logging

import "context"

type requestIDContextKey struct{}

// WithRequestID records a previously validated request ID for downstream
// operational events. Invalid values are deliberately omitted.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if !ValidRequestID(requestID) {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}
