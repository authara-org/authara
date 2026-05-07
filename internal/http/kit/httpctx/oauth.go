package httpctx

import "context"

type oauthNonceKeyType struct{}

var oauthNonceKey = oauthNonceKeyType{}

func WithOAuthNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, oauthNonceKey, nonce)
}

func OAuthNonce(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(oauthNonceKey).(string)
	return v, ok
}

func OAuthNonceOrDefault(ctx context.Context, def string) string {
	if nonce, ok := OAuthNonce(ctx); ok {
		return nonce
	}
	return def
}
