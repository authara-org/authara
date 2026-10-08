package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

const CleanupLeaseName = "cleanup"

var ErrMaintenanceLeaseNotFound = errors.New("maintenance lease not found")

type MaintenanceLease struct {
	Name       string
	OwnerID    uuid.UUID
	LeaseUntil time.Time
	Generation int64
}

func (s *Store) TryAcquireMaintenanceLease(
	ctx context.Context,
	name string,
	ownerID uuid.UUID,
	duration time.Duration,
) (MaintenanceLease, bool, error) {
	var lease MaintenanceLease
	err := s.queryRow(ctx, `
		WITH candidate AS (
			SELECT name
			FROM maintenance_leases
			WHERE name = $1
			  AND (owner_id IS NULL OR owner_id = $2 OR lease_until <= now())
			FOR UPDATE SKIP LOCKED
		)
		UPDATE maintenance_leases AS lease
		SET owner_id = $2,
		    lease_until = now() + make_interval(secs => $3),
		    generation = generation + 1,
		    updated_at = now()
		FROM candidate
		WHERE lease.name = candidate.name
		RETURNING lease.name, lease.owner_id, lease.lease_until, lease.generation
	`, name, ownerID, duration.Seconds()).Scan(
		&lease.Name,
		&lease.OwnerID,
		&lease.LeaseUntil,
		&lease.Generation,
	)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if existsErr := s.queryRow(ctx, `SELECT EXISTS (SELECT 1 FROM maintenance_leases WHERE name = $1)`, name).Scan(&exists); existsErr != nil {
			return MaintenanceLease{}, false, existsErr
		}
		if !exists {
			return MaintenanceLease{}, false, ErrMaintenanceLeaseNotFound
		}
		return MaintenanceLease{}, false, nil
	}
	if err != nil {
		return MaintenanceLease{}, false, err
	}
	return lease, true, nil
}

func (s *Store) RunMaintenanceBatch(
	ctx context.Context,
	lease MaintenanceLease,
	batch func(context.Context) (int64, bool, error),
) (rows int64, more bool, owned bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var valid bool
	err = tx.QueryRowContext(ctx, `
		SELECT true
		FROM maintenance_leases
		WHERE name = $1
		  AND owner_id = $2
		  AND generation = $3
		  AND lease_until > now()
		FOR UPDATE
	`, lease.Name, lease.OwnerID, lease.Generation).Scan(&valid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, false, nil
	}
	if err != nil {
		return 0, false, false, err
	}
	if !valid {
		return 0, false, false, nil
	}

	txCtx := context.WithValue(ctx, DbKey, tx)
	rows, more, err = batch(txCtx)
	if err != nil {
		return 0, false, true, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, true, err
	}
	return rows, more, true, nil
}

func (s *Store) RenewMaintenanceLease(
	ctx context.Context,
	lease MaintenanceLease,
	duration time.Duration,
) (MaintenanceLease, bool, error) {
	var renewed MaintenanceLease
	err := s.queryRow(ctx, `
		UPDATE maintenance_leases
		SET lease_until = now() + make_interval(secs => $4),
		    updated_at = now()
		WHERE name = $1
		  AND owner_id = $2
		  AND generation = $3
		  AND lease_until > now()
		RETURNING name, owner_id, lease_until, generation
	`, lease.Name, lease.OwnerID, lease.Generation, duration.Seconds()).Scan(
		&renewed.Name,
		&renewed.OwnerID,
		&renewed.LeaseUntil,
		&renewed.Generation,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return MaintenanceLease{}, false, nil
	}
	if err != nil {
		return MaintenanceLease{}, false, err
	}
	return renewed, true, nil
}

func (s *Store) ReleaseMaintenanceLease(ctx context.Context, lease MaintenanceLease) (bool, error) {
	result, err := s.exec(ctx, `
		UPDATE maintenance_leases
		SET owner_id = NULL,
		    lease_until = NULL,
		    updated_at = now()
		WHERE name = $1
		  AND owner_id = $2
		  AND generation = $3
	`, lease.Name, lease.OwnerID, lease.Generation)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}
