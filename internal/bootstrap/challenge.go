package bootstrap

import "github.com/authara-org/authara/internal/challenge"

func newVerificationCodeService(app *App) *challenge.VerificationCodeService {
	return challenge.NewVerificationCodeServiceWithPolicy(
		app.Store,
		app.Config,
		app.Config.Challenge.VerificationCodeSecrets...,
	)
}
