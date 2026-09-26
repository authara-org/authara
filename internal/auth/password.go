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

	minimumArgonTime    = 1
	maximumArgonTime    = 10
	minimumArgonMemory  = 8
	maximumArgonMemory  = 256 * 1024
	minimumArgonThreads = 1
	maximumArgonThreads = 16
	minimumSaltLen      = 1
	maximumSaltLen      = 64
	minimumHashLen      = 1
	maximumHashLen      = 64
)

func Hash(password string) (string, error) {
	if err := validatePassword(password, DefaultPasswordMinimumLength); err != nil {
		return "", err
	}
	return hashPassword(password)
}

func hashPassword(password string) (string, error) {
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
	result, err := VerifyPasswordHash(password, encoded)
	return result.Valid, err
}

type PasswordHashVerification struct {
	Valid       bool
	NeedsRehash bool
}

func VerifyPasswordHash(password, encoded string) (PasswordHashVerification, error) {
	params, salt, expected, err := decodeHash(encoded)
	if err != nil {
		return PasswordHashVerification{}, err
	}

	actual := argon2.IDKey(
		[]byte(password),
		salt,
		params.time,
		params.memory,
		params.threads,
		uint32(len(expected)),
	)

	valid := subtle.ConstantTimeCompare(actual, expected) == 1
	return PasswordHashVerification{
		Valid: valid,
		NeedsRehash: valid && (params.time < argonTime ||
			params.memory < argonMemory ||
			params.threads < argonThreads ||
			len(salt) < saltLen ||
			len(expected) < argonKeyLen),
	}, nil
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

	if time < minimumArgonTime || time > maximumArgonTime {
		return nil, nil, nil, errors.New("password hash time cost is out of range")
	}
	if memory < minimumArgonMemory || memory > maximumArgonMemory {
		return nil, nil, nil, errors.New("password hash memory cost is out of range")
	}
	if threads64 < minimumArgonThreads || threads64 > maximumArgonThreads {
		return nil, nil, nil, errors.New("password hash parallelism is out of range")
	}
	if len(salt) < minimumSaltLen || len(salt) > maximumSaltLen {
		return nil, nil, nil, errors.New("password hash salt length is out of range")
	}
	if len(hash) < minimumHashLen || len(hash) > maximumHashLen {
		return nil, nil, nil, errors.New("password hash length is out of range")
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
