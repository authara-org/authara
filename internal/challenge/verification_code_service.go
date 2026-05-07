package challenge

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

type VerificationCodeService struct {
	store   *store.Store
	ttl     time.Duration
	secrets [][]byte
}

func NewVerificationCodeService(store *store.Store, ttl time.Duration, secrets ...[]byte) *VerificationCodeService {
	return &VerificationCodeService{
		store:   store,
		ttl:     ttl,
		secrets: cloneSecrets(secrets),
	}
}

func (s *VerificationCodeService) GenerateCode(
	ctx context.Context,
	challenge domain.Challenge,
	now time.Time,
) (string, error) {
	if len(s.secrets) == 0 {
		return "", errors.New("verification code secret is not configured")
	}

	code, err := generateSixDigitCode()
	if err != nil {
		return "", err
	}

	expiresAt := now.Add(s.ttl)
	if expiresAt.After(challenge.ExpiresAt) {
		expiresAt = challenge.ExpiresAt
	}

	err = s.store.UpsertVerificationCode(ctx, domain.VerificationCode{
		ChallengeID: challenge.ID,
		CodeHash:    hashVerificationCode(code, s.secrets[0]),
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return "", err
	}

	return code, nil
}

func (s *VerificationCodeService) VerifyCode(
	ctx context.Context,
	challengeID uuid.UUID,
	code string,
	now time.Time,
) error {
	row, err := s.store.GetVerificationCodeByChallengeID(ctx, challengeID)
	if err != nil {
		return err
	}

	if row.ExpiresAt.Before(now) {
		return ErrChallengeExpired
	}

	if len(s.secrets) == 0 {
		return errors.New("verification code secret is not configured")
	}

	if !s.matchesVerificationCodeHash(code, row.CodeHash) {
		return ErrInvalidVerificationCode
	}

	return nil
}

func generateSixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (s *VerificationCodeService) matchesVerificationCodeHash(code string, storedHash string) bool {
	for _, secret := range s.secrets {
		got := hashVerificationCode(code, secret)
		if subtle.ConstantTimeCompare([]byte(got), []byte(storedHash)) == 1 {
			return true
		}
	}
	return false
}

func hashVerificationCode(code string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func cloneSecrets(secrets [][]byte) [][]byte {
	out := make([][]byte, 0, len(secrets))
	for _, secret := range secrets {
		if len(secret) == 0 {
			continue
		}
		cloned := append([]byte(nil), secret...)
		out = append(out, cloned)
	}
	return out
}
