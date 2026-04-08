package httpctx

import "context"

type challengeEnabledKey struct{}

func WithChallengeEnabled(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, challengeEnabledKey{}, enabled)
}

func ChallengeEnabled(ctx context.Context) (bool, bool) {
	v, ok := ctx.Value(challengeEnabledKey{}).(bool)
	return v, ok
}

func ChallengeEnabledOrDefault(ctx context.Context, fallback bool) bool {
	v, ok := ChallengeEnabled(ctx)
	if !ok {
		return fallback
	}
	return v
}
