package securityevent

import "context"

type NoopRecorder struct{}

func (NoopRecorder) AuthenticationLogin(context.Context, Authentication) error           { return nil }
func (NoopRecorder) AuthenticationReauthenticated(context.Context, Authentication) error { return nil }
func (NoopRecorder) SessionRefresh(context.Context, Session) error                       { return nil }
func (NoopRecorder) SessionRefreshTokenReuse(context.Context, Session) error             { return nil }
func (NoopRecorder) SessionLogout(context.Context, Session) error                        { return nil }
func (NoopRecorder) SessionRevoked(context.Context, Session) error                       { return nil }
func (NoopRecorder) CredentialPasswordAdded(context.Context, Credential) error           { return nil }
func (NoopRecorder) CredentialPasswordChanged(context.Context, Credential) error         { return nil }
func (NoopRecorder) CredentialPasswordReset(context.Context, Credential) error           { return nil }
func (NoopRecorder) CredentialProviderLinked(context.Context, Credential) error          { return nil }
func (NoopRecorder) CredentialProviderChanged(context.Context, Credential) error         { return nil }
func (NoopRecorder) CredentialProviderRemoved(context.Context, Credential) error         { return nil }
func (NoopRecorder) CredentialPasskeyAdded(context.Context, Credential) error            { return nil }
func (NoopRecorder) CredentialPasskeyRemoved(context.Context, Credential) error          { return nil }
func (NoopRecorder) AccountEmailChanged(context.Context, Credential) error               { return nil }
func (NoopRecorder) PasskeyCloneWarning(context.Context, PasskeyClone) error             { return nil }
