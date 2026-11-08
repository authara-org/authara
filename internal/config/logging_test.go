package config

import "testing"

func TestLoggingEnvironmentIsBoundedAndValidatedWithoutExplicitLevel(t *testing.T) {
	for _, environment := range []string{"staging", "eu-central-1", "preview_42"} {
		logging := Logging{Environment: environment}
		if err := logging.validate(); err != nil {
			t.Errorf("environment %q: %v", environment, err)
		}
	}
	for _, environment := range []string{"contains space", "../production", "customer@example.com", "", "abcdefghijklmnopqrstuvwxyz1234567"} {
		logging := Logging{Environment: environment}
		err := logging.validate()
		if environment == "" {
			if err != nil {
				t.Errorf("empty environment should use APP_ENV: %v", err)
			}
			continue
		}
		if err == nil {
			t.Errorf("invalid environment %q accepted", environment)
		}
	}
}
