package store

import (
	"context"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainPasskey(m model.Passkey) domain.Passkey {
	return domain.Passkey{
		ID:                m.ID,
		UserID:            m.UserID,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
		CredentialID:      m.CredentialID,
		PublicKey:         m.PublicKey,
		AttestationType:   m.AttestationType,
		AttestationFormat: m.AttestationFormat,
		Transport:         splitTransport(m.Transport),
		AAGUID:            m.AAGUID,
		SignCount:         uint32(m.SignCount),
		CloneWarning:      m.CloneWarning,
		Name:              m.Name,
		LastUsedAt:        m.LastUsedAt,
		UserPresent:       m.UserPresent,
		UserVerified:      m.UserVerified,
		BackupEligible:    m.BackupEligible,
		BackupState:       m.BackupState,
	}
}

func toModelPasskey(d domain.Passkey) model.Passkey {
	return model.Passkey{
		UserID:            d.UserID,
		CredentialID:      d.CredentialID,
		PublicKey:         d.PublicKey,
		AttestationType:   d.AttestationType,
		AttestationFormat: d.AttestationFormat,
		Transport:         joinTransport(d.Transport),
		AAGUID:            d.AAGUID,
		SignCount:         int64(d.SignCount),
		CloneWarning:      d.CloneWarning,
		Name:              d.Name,
		LastUsedAt:        d.LastUsedAt,
		UserPresent:       d.UserPresent,
		UserVerified:      d.UserVerified,
		BackupEligible:    d.BackupEligible,
		BackupState:       d.BackupState,
	}
}

const passkeyColumns = `
	id,
	user_id,
	credential_id,
	public_key,
	attestation_type,
	attestation_format,
	array_to_string(transport, ',') AS transport,
	aaguid,
	sign_count,
	clone_warning,
	name,
	created_at,
	updated_at,
	last_used_at,
	user_present,
	user_verified,
	backup_eligible,
	backup_state
`

func scanPasskey(row rowScanner, m *model.Passkey) error {
	return row.Scan(
		&m.ID,
		&m.UserID,
		&m.CredentialID,
		&m.PublicKey,
		&m.AttestationType,
		&m.AttestationFormat,
		&m.Transport,
		&m.AAGUID,
		&m.SignCount,
		&m.CloneWarning,
		&m.Name,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.LastUsedAt,
		&m.UserPresent,
		&m.UserVerified,
		&m.BackupEligible,
		&m.BackupState,
	)
}

func (s *Store) CreatePasskey(ctx context.Context, passkey domain.Passkey) (domain.Passkey, error) {
	m := toModelPasskey(passkey)
	if m.Name == "" {
		m.Name = "Passkey"
	}

	if err := scanPasskey(s.queryRow(ctx, `
		INSERT INTO passkeys (
			user_id,
			credential_id,
			public_key,
			attestation_type,
			attestation_format,
			transport,
			aaguid,
			sign_count,
			clone_warning,
			name,
			last_used_at,
			user_present,
			user_verified,
			backup_eligible,
			backup_state
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			CASE WHEN $6 = '' THEN '{}'::text[] ELSE string_to_array($6, ',') END,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14,
			$15
		)
		RETURNING `+passkeyColumns,
		m.UserID,
		m.CredentialID,
		m.PublicKey,
		m.AttestationType,
		m.AttestationFormat,
		m.Transport,
		m.AAGUID,
		m.SignCount,
		m.CloneWarning,
		m.Name,
		m.LastUsedAt,
		m.UserPresent,
		m.UserVerified,
		m.BackupEligible,
		m.BackupState,
	), &m); err != nil {
		if IsUniqueViolation(err, ConstraintPasskeyCredentialID) {
			return domain.Passkey{}, ErrPasskeyAlreadyExists
		}
		return domain.Passkey{}, err
	}

	return toDomainPasskey(m), nil
}

func (s *Store) ListPasskeysByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Passkey, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+passkeyColumns+`
		FROM passkeys
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Passkey, 0)
	for rows.Next() {
		var row model.Passkey
		if err := scanPasskey(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, toDomainPasskey(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) GetPasskeyByCredentialID(ctx context.Context, credentialID []byte) (domain.Passkey, error) {
	var m model.Passkey

	err := scanPasskey(s.queryRow(ctx, `
		SELECT `+passkeyColumns+`
		FROM passkeys
		WHERE credential_id = $1
	`, credentialID), &m)
	if err != nil {
		return domain.Passkey{}, mapNoRows(err, ErrPasskeyNotFound)
	}

	return toDomainPasskey(m), nil
}

func (s *Store) GetPasskeyByID(ctx context.Context, passkeyID uuid.UUID) (domain.Passkey, error) {
	var m model.Passkey

	err := scanPasskey(s.queryRow(ctx, `
		SELECT `+passkeyColumns+`
		FROM passkeys
		WHERE id = $1
	`, passkeyID), &m)
	if err != nil {
		return domain.Passkey{}, mapNoRows(err, ErrPasskeyNotFound)
	}

	return toDomainPasskey(m), nil
}

func (s *Store) DeletePasskeyByIDAndUserID(ctx context.Context, passkeyID, userID uuid.UUID) error {
	res, err := s.exec(ctx, `
		DELETE FROM passkeys
		WHERE id = $1 AND user_id = $2
	`, passkeyID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrPasskeyNotFound
	}

	return nil
}

func (s *Store) UpdatePasskeyAfterLogin(ctx context.Context, credentialID []byte, signCount uint32, cloneWarning bool, now time.Time) error {
	res, err := s.exec(ctx, `
		UPDATE passkeys
		SET sign_count = $1,
		    clone_warning = $2,
		    last_used_at = $3
		WHERE credential_id = $4
	`, int64(signCount), cloneWarning, now, credentialID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrPasskeyNotFound
	}

	return nil
}

func (s *Store) CountAuthMethods(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.queryRow(ctx, `
		SELECT
			(SELECT count(*)
			   FROM auth_providers
			  WHERE user_id = $1
			    AND (
			      (provider = $2 AND password_hash IS NOT NULL AND password_hash <> '')
			      OR
			      (provider <> $2 AND provider_user_id IS NOT NULL AND provider_user_id <> '')
			    )
			) +
			(SELECT count(*) FROM passkeys WHERE user_id = $1)
	`, userID, string(domain.ProviderPassword)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CreateWebAuthnChallenge(ctx context.Context, in domain.WebAuthnChallenge) (domain.WebAuthnChallenge, error) {
	m := model.WebAuthnChallenge{
		UserID:      in.UserID,
		Purpose:     string(in.Purpose),
		Challenge:   in.Challenge,
		SessionData: in.SessionData,
		ExpiresAt:   in.ExpiresAt,
		ConsumedAt:  in.ConsumedAt,
	}

	if err := scanWebAuthnChallenge(s.queryRow(ctx, `
		INSERT INTO webauthn_challenges (
			user_id,
			purpose,
			challenge,
			session_data,
			expires_at,
			consumed_at
		)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
		RETURNING `+webAuthnChallengeColumns,
		m.UserID,
		m.Purpose,
		m.Challenge,
		string(m.SessionData),
		m.ExpiresAt,
		m.ConsumedAt,
	), &m); err != nil {
		return domain.WebAuthnChallenge{}, err
	}

	return toDomainWebAuthnChallenge(m), nil
}

func (s *Store) GetWebAuthnChallengeByIDForUpdate(ctx context.Context, challengeID uuid.UUID) (domain.WebAuthnChallenge, error) {
	var m model.WebAuthnChallenge

	err := scanWebAuthnChallenge(s.queryRow(ctx, `
		SELECT `+webAuthnChallengeColumns+`
		FROM webauthn_challenges
		WHERE id = $1
		FOR UPDATE
	`, challengeID), &m)
	if err != nil {
		return domain.WebAuthnChallenge{}, mapNoRows(err, ErrWebAuthnChallengeNotFound)
	}

	return toDomainWebAuthnChallenge(m), nil
}

func (s *Store) ConsumeWebAuthnChallenge(ctx context.Context, challengeID uuid.UUID, now time.Time) error {
	res, err := s.exec(ctx, `
		UPDATE webauthn_challenges
		SET consumed_at = $1
		WHERE id = $2 AND consumed_at IS NULL
	`, now, challengeID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrWebAuthnChallengeAlreadyConsumed
	}
	return nil
}

func (s *Store) DeleteExpiredWebAuthnChallenges(ctx context.Context, now time.Time) error {
	_, err := s.exec(ctx, `
		DELETE FROM webauthn_challenges
		WHERE expires_at < $1 OR consumed_at IS NOT NULL
	`, now)
	return err
}

const webAuthnChallengeColumns = `
	id,
	created_at,
	user_id,
	purpose,
	challenge,
	session_data,
	expires_at,
	consumed_at
`

func scanWebAuthnChallenge(row rowScanner, m *model.WebAuthnChallenge) error {
	err := row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UserID,
		&m.Purpose,
		&m.Challenge,
		&m.SessionData,
		&m.ExpiresAt,
		&m.ConsumedAt,
	)
	return err
}

func toDomainWebAuthnChallenge(m model.WebAuthnChallenge) domain.WebAuthnChallenge {
	return domain.WebAuthnChallenge{
		ID:          m.ID,
		CreatedAt:   m.CreatedAt,
		UserID:      m.UserID,
		Purpose:     domain.WebAuthnChallengePurpose(m.Purpose),
		Challenge:   m.Challenge,
		SessionData: m.SessionData,
		ExpiresAt:   m.ExpiresAt,
		ConsumedAt:  m.ConsumedAt,
	}
}

func splitTransport(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func joinTransport(v []string) string {
	if len(v) == 0 {
		return ""
	}
	parts := make([]string, 0, len(v))
	for _, part := range v {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ",")
}
