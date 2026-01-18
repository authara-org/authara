package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

/*
Password hash format
$argon2id$v=1$t=3$m=65536$p=4$<salt>$<hash>
*/

const (
	algorithm = "argon2id"
	version   = 1

	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32

	saltLen = 16
)

func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		argonKeyLen,
	)

	return encodeHash(
		argonTime,
		argonMemory,
		argonThreads,
		salt,
		hash,
	), nil
}

func Verify(password, encoded string) (bool, error) {
	params, salt, expected, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}

	actual := argon2.IDKey(
		[]byte(password),
		salt,
		params.time,
		params.memory,
		params.threads,
		uint32(len(expected)),
	)

	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

type parameters struct {
	time    uint32
	memory  uint32
	threads uint8
}

func encodeHash(
	time uint32,
	memory uint32,
	threads uint8,
	salt []byte,
	hash []byte,
) string {
	return fmt.Sprintf(
		"$%s$v=%d$t=%d$m=%d$p=%d$%s$%s",
		algorithm,
		version,
		time,
		memory,
		threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

func decodeHash(encoded string) (*parameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 8 {
		return nil, nil, nil, errors.New("invalid password hash format")
	}

	if parts[1] != algorithm {
		return nil, nil, nil, errors.New("unsupported password algorithm")
	}

	if parts[2] != "v=1" {
		return nil, nil, nil, errors.New("unsupported password hash version")
	}

	time, err := parseUint(parts[3], "t")
	if err != nil {
		return nil, nil, nil, err
	}

	memory, err := parseUint(parts[4], "m")
	if err != nil {
		return nil, nil, nil, err
	}

	threads64, err := parseUint(parts[5], "p")
	if err != nil {
		return nil, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[6])
	if err != nil {
		return nil, nil, nil, err
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[7])
	if err != nil {
		return nil, nil, nil, err
	}

	return &parameters{
		time:    uint32(time),
		memory:  uint32(memory),
		threads: uint8(threads64),
	}, salt, hash, nil
}

func parseUint(s, prefix string) (uint64, error) {
	if !strings.HasPrefix(s, prefix+"=") {
		return 0, errors.New("invalid password hash parameter")
	}
	return strconv.ParseUint(strings.TrimPrefix(s, prefix+"="), 10, 32)
}
