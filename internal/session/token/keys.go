package token

import (
	"errors"
)

type KeySet struct {
	ActiveKeyID string
	Keys        map[string][]byte
}

func NewKeySet(activeKeyID string, keys map[string][]byte) (*KeySet, error) {
	if activeKeyID == "" {
		return nil, errors.New("active key id must be set")
	}

	key, ok := keys[activeKeyID]
	if !ok || len(key) == 0 {
		return nil, errors.New("active signing key not found")
	}

	return &KeySet{
		ActiveKeyID: activeKeyID,
		Keys:        keys,
	}, nil
}

func (k *KeySet) SigningKey() (keyID string, key []byte) {
	return k.ActiveKeyID, k.Keys[k.ActiveKeyID]
}

func (k *KeySet) VerificationKey(keyID string) ([]byte, bool) {
	key, ok := k.Keys[keyID]
	return key, ok
}
