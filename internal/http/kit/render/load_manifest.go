package render

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func LoadAssetsManifest(path string) (Assets, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// dev mode fallback (no fingerprinting)
			return Assets{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var m map[string]string
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode assets manifest %q: %w", path, err)
	}

	return Assets(m), nil
}
