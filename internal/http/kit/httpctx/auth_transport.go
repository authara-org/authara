package httpctx

import "context"

type AuthTransport string

const (
	AuthTransportCookie AuthTransport = "cookie"
	AuthTransportBearer AuthTransport = "bearer"
)

type authTransportKeyType struct{}

var authTransportKey authTransportKeyType

func WithAuthTransport(ctx context.Context, transport AuthTransport) context.Context {
	return context.WithValue(ctx, authTransportKey, transport)
}

func AuthenticationTransport(ctx context.Context) (AuthTransport, bool) {
	transport, ok := ctx.Value(authTransportKey).(AuthTransport)
	return transport, ok
}
