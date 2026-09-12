package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	configruntime "github.com/authara-org/authara/internal/config/runtime"
	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

func (s *Store) LoadRuntimeSettings(ctx context.Context) (configruntime.PersistedState, error) {
	rows, err := s.queryRows(ctx, `
		SELECT
			state.revision,
			override.setting_key,
			override.value,
			override.revision,
			override.created_at,
			override.updated_at,
			override.updated_by_user_id
		FROM runtime_settings_state AS state
		LEFT JOIN runtime_setting_overrides AS override ON true
		WHERE state.singleton = true
		ORDER BY override.setting_key ASC
	`)
	if err != nil {
		return configruntime.PersistedState{}, err
	}
	defer rows.Close()

	var state configruntime.PersistedState
	foundState := false
	for rows.Next() {
		foundState = true
		var key sql.NullString
		var value []byte
		var revision sql.NullInt64
		var createdAt, updatedAt sql.NullTime
		var updatedByUserID *uuid.UUID
		if err := rows.Scan(
			&state.Revision,
			&key,
			&value,
			&revision,
			&createdAt,
			&updatedAt,
			&updatedByUserID,
		); err != nil {
			return configruntime.PersistedState{}, err
		}
		if !key.Valid {
			continue
		}
		state.Overrides = append(state.Overrides, configruntime.PersistedOverride{
			Key: configruntime.Key(key.String), Value: append(json.RawMessage(nil), value...), Revision: revision.Int64,
			CreatedAt: createdAt.Time, UpdatedAt: updatedAt.Time, UpdatedByUserID: updatedByUserID,
		})
	}
	if err := rows.Err(); err != nil {
		return configruntime.PersistedState{}, err
	}
	if !foundState {
		return configruntime.PersistedState{}, fmt.Errorf("runtime settings state row is missing")
	}
	return state, nil
}

func (s *Store) UpsertRuntimeSettingOverride(ctx context.Context, mutation configruntime.Mutation) (configruntime.PersistedOverride, error) {
	var saved configruntime.PersistedOverride
	var key string
	var value []byte
	var updatedByUserID *uuid.UUID
	err := s.queryRow(ctx, `
		WITH existing AS MATERIALIZED (
			SELECT setting_key, value, revision
			FROM runtime_setting_overrides
			WHERE setting_key = $1
			FOR UPDATE
		), next_revision AS MATERIALIZED (
			SELECT revision + 1 AS revision
			FROM runtime_settings_state
			WHERE singleton = true AND revision = $8
			FOR UPDATE
		), saved AS (
			INSERT INTO runtime_setting_overrides (
				setting_key, value, revision, updated_by_user_id
			)
			SELECT $1, $2::jsonb, next_revision.revision, $4
			FROM next_revision
			WHERE ($3 = 0 AND NOT EXISTS (SELECT 1 FROM existing))
			   OR EXISTS (SELECT 1 FROM existing WHERE revision = $3)
			ON CONFLICT (setting_key) DO UPDATE
			SET value = EXCLUDED.value,
			    revision = EXCLUDED.revision,
			    updated_by_user_id = EXCLUDED.updated_by_user_id
			WHERE runtime_setting_overrides.revision = $3
			RETURNING setting_key, value, revision, created_at, updated_at, updated_by_user_id
		), bumped AS (
			UPDATE runtime_settings_state AS state
			SET revision = saved.revision
			FROM saved
			WHERE state.singleton = true
			RETURNING state.revision
		), audited AS (
			INSERT INTO operator_audit_events (
				actor_user_id, action, resource_type, resource_id, metadata
			)
			SELECT $4, $5, $6, saved.setting_key,
			       $7::jsonb || jsonb_build_object('revision', saved.revision)
			FROM saved
			WHERE EXISTS (SELECT 1 FROM bumped)
			RETURNING id
		)
		SELECT saved.setting_key, saved.value, saved.revision,
		       saved.created_at, saved.updated_at, saved.updated_by_user_id
		FROM saved
		WHERE EXISTS (SELECT 1 FROM audited)
	`,
		string(mutation.Key), string(mutation.Value), mutation.ExpectedRevision,
		mutation.ActorUserID, domain.OperatorAuditActionRuntimeSettingSet,
		domain.OperatorAuditResourceRuntimeSetting, string(mutation.AuditMetadata),
		mutation.ExpectedStateRevision,
	).Scan(&key, &value, &saved.Revision, &saved.CreatedAt, &saved.UpdatedAt, &updatedByUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return configruntime.PersistedOverride{}, configruntime.ErrRevisionConflict
	}
	if err != nil {
		return configruntime.PersistedOverride{}, err
	}
	saved.Key = configruntime.Key(key)
	saved.Value = append(json.RawMessage(nil), value...)
	saved.UpdatedByUserID = updatedByUserID
	return saved, nil
}

func (s *Store) DeleteRuntimeSettingOverride(
	ctx context.Context,
	key configruntime.Key,
	expectedRevision int64,
	expectedStateRevision int64,
	actorUserID uuid.UUID,
	auditMetadata json.RawMessage,
) (int64, error) {
	var revision int64
	err := s.queryRow(ctx, `
		WITH existing AS MATERIALIZED (
			SELECT setting_key, value, revision
			FROM runtime_setting_overrides
			WHERE setting_key = $1 AND revision = $2
			FOR UPDATE
		), next_revision AS MATERIALIZED (
			SELECT revision + 1 AS revision
			FROM runtime_settings_state
			WHERE singleton = true AND revision = $3
			FOR UPDATE
		), deleted AS (
			DELETE FROM runtime_setting_overrides AS override
			USING existing, next_revision
			WHERE override.setting_key = existing.setting_key
			RETURNING override.setting_key
		), bumped AS (
			UPDATE runtime_settings_state AS state
			SET revision = next_revision.revision
			FROM deleted, next_revision
			WHERE state.singleton = true
			RETURNING state.revision
		), audited AS (
			INSERT INTO operator_audit_events (
				actor_user_id, action, resource_type, resource_id, metadata
			)
			SELECT $4, $5, $6, deleted.setting_key,
			       $7::jsonb || jsonb_build_object('revision', bumped.revision)
			FROM deleted
			CROSS JOIN bumped
			RETURNING id
		)
		SELECT bumped.revision
		FROM bumped
		WHERE EXISTS (SELECT 1 FROM audited)
	`,
		string(key), expectedRevision, expectedStateRevision, actorUserID,
		domain.OperatorAuditActionRuntimeSettingCleared,
		domain.OperatorAuditResourceRuntimeSetting,
		string(auditMetadata),
	).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, configruntime.ErrRevisionConflict
	}
	return revision, err
}
