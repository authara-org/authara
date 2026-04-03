package challenge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

type VerificationCodeService struct {
	store *store.Store
	ttl   time.Duration
}

func NewVerificationCodeService(store *store.Store, ttl time.Duration) *VerificationCodeService {
	return &VerificationCodeService{
		store: store,
		ttl:   ttl,
	}
}

func (s *VerificationCodeService) GenerateCode(
	ctx context.Context,
	challenge domain.Challenge,
	now time.Time,
) (string, error) {
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
		CodeHash:    hashVerificationCode(code),
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

	got := hashVerificationCode(code)
	if subtle.ConstantTimeCompare([]byte(got), []byte(row.CodeHash)) != 1 {
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

func hashVerificationCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
