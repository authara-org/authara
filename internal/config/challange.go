package config

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Challenge struct {
	Enabled                     bool              `env:"AUTHARA_CHALLENGE_ENABLED,default=false"`
	TTL                         time.Duration     `env:"AUTHARA_CHALLENGE_TTL,default=30m"`
	VerificationCodeTTL         time.Duration     `env:"AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL,default=10m"`
	MaxAttempts                 int               `env:"AUTHARA_CHALLENGE_MAX_ATTEMPTS,default=5"`
	MaxResends                  int               `env:"AUTHARA_CHALLENGE_MAX_RESENDS,default=3"`
	MinResendInterval           time.Duration     `env:"AUTHARA_CHALLENGE_MIN_RESEND_INTERVAL,default=30s"`
	VerificationCodeActiveKeyID string            `env:"AUTHARA_CHALLENGE_VERIFICATION_CODE_ACTIVE_KEY_ID,required"`
	VerificationCodeKeys        map[string]string `env:"AUTHARA_CHALLENGE_VERIFICATION_CODE_KEYS,required"`

	VerificationCodeSecrets [][]byte
}

func (c *Challenge) validate() error {
	if c.TTL <= 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_TTL must be > 0")
	}
	if c.VerificationCodeTTL <= 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL must be > 0")
	}
	// The relationship between code and challenge lifetimes is validated after
	// runtime overrides have been loaded. Comparing parser-resolved values here
	// would reject a valid hybrid environment/database combination too early.
	if c.MaxAttempts <= 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_MAX_ATTEMPTS must be > 0")
	}
	if c.MaxResends < 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_MAX_RESENDS must be >= 0")
	}
	if c.MinResendInterval < 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_MIN_RESEND_INTERVAL must be >= 0")
	}
	if strings.TrimSpace(c.VerificationCodeActiveKeyID) == "" {
		return fmt.Errorf("AUTHARA_CHALLENGE_VERIFICATION_CODE_ACTIVE_KEY_ID must not be empty")
	}
	if len(c.VerificationCodeKeys) == 0 {
		return fmt.Errorf("AUTHARA_CHALLENGE_VERIFICATION_CODE_KEYS must contain at least one key")
	}
	if key, ok := c.VerificationCodeKeys[c.VerificationCodeActiveKeyID]; !ok || key == "" {
		return fmt.Errorf(
			"AUTHARA_CHALLENGE_VERIFICATION_CODE_ACTIVE_KEY_ID %q not found in AUTHARA_CHALLENGE_VERIFICATION_CODE_KEYS",
			c.VerificationCodeActiveKeyID,
		)
	}

	return nil
}

func (c *Challenge) parse() error {
	keyIDs := make([]string, 0, len(c.VerificationCodeKeys))
	for keyID := range c.VerificationCodeKeys {
		if keyID != c.VerificationCodeActiveKeyID {
			keyIDs = append(keyIDs, keyID)
		}
	}
	sort.Strings(keyIDs)
	keyIDs = append([]string{c.VerificationCodeActiveKeyID}, keyIDs...)

	secrets := make([][]byte, 0, len(keyIDs))
	for _, keyID := range keyIDs {
		secret, err := base64.StdEncoding.Strict().DecodeString(c.VerificationCodeKeys[keyID])
		if err != nil {
			return fmt.Errorf("AUTHARA_CHALLENGE_VERIFICATION_CODE_KEYS[%q] is not valid base64", keyID)
		}
		if len(secret) < 32 {
			return fmt.Errorf("AUTHARA_CHALLENGE_VERIFICATION_CODE_KEYS[%q] must decode to at least 32 bytes", keyID)
		}
		secrets = append(secrets, secret)
	}
	c.VerificationCodeSecrets = secrets
	return nil
}
