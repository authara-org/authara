package bootstrap

import "github.com/authara-org/authara/internal/challenge"

func newVerificationCodeService(app *App) *challenge.VerificationCodeService {
	_, activeVerificationSecret := app.Config.Token.KeySet.SigningKey()
	verificationSecrets := [][]byte{activeVerificationSecret}
	for keyID, secret := range app.Config.Token.KeySet.Keys {
		if keyID == app.Config.Token.KeySet.ActiveKeyID {
			continue
		}
		verificationSecrets = append(verificationSecrets, secret)
	}

	return challenge.NewVerificationCodeService(
		app.Store,
		app.Config.Challenge.VerificationCodeTTL,
		verificationSecrets...,
	)
}
