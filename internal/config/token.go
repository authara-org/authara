package config

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/authara-org/authara/internal/session/token"
)

type Token struct {
	Issuer                string            `env:"AUTHARA_JWT_ISSUER,required"`
	ActiveKeyID           string            `env:"AUTHARA_JWT_ACTIVE_KEY_ID,required"`
	Keys                  map[string]string `env:"AUTHARA_JWT_KEYS,required"`
	AccessTokenTTLMinutes int               `env:"AUTHARA_ACCESS_TOKEN_TTL_MINUTES,default=10"`

	AccessTokenTTL time.Duration
	KeySet         *token.KeySet
}

func (t *Token) validate() error {
	if t.Issuer == "" {
		return fmt.Errorf("AUTHARA_JWT_ISSUER must not be empty")
	}

	if t.ActiveKeyID == "" {
		return fmt.Errorf("AUTHARA_JWT_ACTIVE_KEY_ID must not be empty")
	}

	if len(t.Keys) == 0 {
		return fmt.Errorf("AUTHARA_JWT_KEYS must contain at least one key")
	}

	key, ok := t.Keys[t.ActiveKeyID]
	if !ok {
		return fmt.Errorf(
			"AUTHARA_JWT_ACTIVE_KEY_ID %q not found in AUTHARA_JWT_KEYS",
			t.ActiveKeyID,
		)
	}

	if key == "" {
		return fmt.Errorf(
			"AUTHARA_JWT_KEYS[%q] must not be empty",
			t.ActiveKeyID,
		)
	}

	if t.AccessTokenTTLMinutes <= 0 {
		return fmt.Errorf(
			"AUTHARA_ACCESS_TOKEN_TTL_MINUTES must be greater than 0 (got %d)",
			t.AccessTokenTTLMinutes,
		)
	}

	return nil
}

func (t *Token) parse() error {
	t.AccessTokenTTL = time.Duration(t.AccessTokenTTLMinutes) * time.Minute

	decoded := make(map[string][]byte, len(t.Keys))

	for id, encoded := range t.Keys {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("AUTHARA_JWT_KEYS[%q] is not valid base64", id)
		}
		decoded[id] = key
	}

	keySet, err := token.NewKeySet(t.ActiveKeyID, decoded)
	if err != nil {
		return err
	}

	t.KeySet = keySet
	return nil
}
