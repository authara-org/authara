package token

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type KeySet struct {
	activeKeyID string
	privateKeys map[string]ed25519.PrivateKey
	publicKeys  map[string]ed25519.PublicKey
}

type JSONWebKeySet struct {
	Keys []JSONWebKey `json:"keys"`
}

type JSONWebKey struct {
	KeyType   string   `json:"kty"`
	KeyID     string   `json:"kid"`
	Use       string   `json:"use"`
	Algorithm string   `json:"alg"`
	Curve     string   `json:"crv"`
	X         string   `json:"x"`
	KeyOps    []string `json:"key_ops"`
}

// NewKeySet constructs an Ed25519 signing key set from 32-byte private-key
// seeds. Public verification keys are derived and kept separately so they can
// be published without exposing private material.
func NewKeySet(activeKeyID string, seeds map[string][]byte) (*KeySet, error) {
	if strings.TrimSpace(activeKeyID) == "" {
		return nil, errors.New("active key id must be set")
	}

	if _, ok := seeds[activeKeyID]; !ok {
		return nil, errors.New("active signing key not found")
	}

	privateKeys := make(map[string]ed25519.PrivateKey, len(seeds))
	publicKeys := make(map[string]ed25519.PublicKey, len(seeds))
	for keyID, seed := range seeds {
		if strings.TrimSpace(keyID) == "" {
			return nil, errors.New("key id must not be empty")
		}
		if len(seed) != ed25519.SeedSize {
			return nil, fmt.Errorf("signing key %q must be a %d-byte Ed25519 seed", keyID, ed25519.SeedSize)
		}
		privateKey := ed25519.NewKeyFromSeed(seed)
		publicKey := privateKey.Public().(ed25519.PublicKey)
		privateKeys[keyID] = append(ed25519.PrivateKey(nil), privateKey...)
		publicKeys[keyID] = append(ed25519.PublicKey(nil), publicKey...)
	}

	return &KeySet{
		activeKeyID: activeKeyID,
		privateKeys: privateKeys,
		publicKeys:  publicKeys,
	}, nil
}

func (k *KeySet) ActiveKeyID() string {
	return k.activeKeyID
}

func (k *KeySet) SigningKey() (keyID string, key ed25519.PrivateKey) {
	return k.activeKeyID, append(ed25519.PrivateKey(nil), k.privateKeys[k.activeKeyID]...)
}

func (k *KeySet) VerificationKey(keyID string) (ed25519.PublicKey, bool) {
	key, ok := k.publicKeys[keyID]
	if !ok {
		return nil, false
	}
	return append(ed25519.PublicKey(nil), key...), true
}

func (k *KeySet) JWKS() JSONWebKeySet {
	keyIDs := make([]string, 0, len(k.publicKeys))
	for keyID := range k.publicKeys {
		keyIDs = append(keyIDs, keyID)
	}
	sort.Strings(keyIDs)

	keys := make([]JSONWebKey, 0, len(keyIDs))
	for _, keyID := range keyIDs {
		keys = append(keys, JSONWebKey{
			KeyType:   "OKP",
			KeyID:     keyID,
			Use:       "sig",
			Algorithm: "EdDSA",
			Curve:     "Ed25519",
			X:         base64.RawURLEncoding.EncodeToString(k.publicKeys[keyID]),
			KeyOps:    []string{"verify"},
		})
	}

	return JSONWebKeySet{Keys: keys}
}
