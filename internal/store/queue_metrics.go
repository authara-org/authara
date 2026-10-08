package store

import (
	"context"
	"database/sql"
	"time"
)

const (
	QueueStatePending    = "pending"
	QueueStateRetry      = "retry"
	QueueStateProcessing = "processing"
	QueueStateFailed     = "failed"
)

type QueueStateStats struct {
	State           string
	Count           int64
	OldestCreatedAt *time.Time
}

type QueueStats struct {
	States        []QueueStateStats
	OldestReadyAt *time.Time
	Stuck         int64
}

type ReapResult struct {
	Retried int64
	Failed  int64
}

func (r ReapResult) Total() int64 { return r.Retried + r.Failed }

func (s *Store) EmailQueueStats(ctx context.Context, now, staleBefore time.Time) (QueueStats, error) {
	return s.queueStats(ctx, `
		WITH pending AS (
			SELECT
				count(*) FILTER (WHERE attempt_count = 0) AS pending_count,
				min(created_at) FILTER (WHERE attempt_count = 0) AS pending_oldest,
				count(*) FILTER (WHERE attempt_count > 0) AS retry_count,
				min(created_at) FILTER (WHERE attempt_count > 0) AS retry_oldest,
				min(next_attempt_at) FILTER (WHERE next_attempt_at <= $1) AS oldest_ready
			FROM email_jobs
			WHERE status = 'pending'
		), processing AS (
			SELECT
				count(*) AS processing_count,
				min(created_at) AS processing_oldest,
				count(*) FILTER (WHERE processing_started_at <= $2) AS stuck
			FROM email_jobs
			WHERE status = 'processing'
		), failed AS (
			SELECT count(*) AS failed_count, min(created_at) AS failed_oldest
			FROM email_jobs
			WHERE status = 'failed'
		)
		SELECT
			pending_count, pending_oldest, retry_count, retry_oldest,
			processing_count, processing_oldest, failed_count, failed_oldest,
			oldest_ready, stuck
		FROM pending CROSS JOIN processing CROSS JOIN failed
	`, now, staleBefore)
}

func (s *Store) WebhookQueueStats(ctx context.Context, now, staleBefore time.Time) (QueueStats, error) {
	return s.queueStats(ctx, `
		WITH pending AS (
			SELECT
				count(*) FILTER (WHERE attempt_count = 0) AS pending_count,
				min(created_at) FILTER (WHERE attempt_count = 0) AS pending_oldest,
				count(*) FILTER (WHERE attempt_count > 0) AS retry_count,
				min(created_at) FILTER (WHERE attempt_count > 0) AS retry_oldest,
				min(next_attempt_at) FILTER (WHERE next_attempt_at <= $1) AS oldest_ready
			FROM webhook_events
			WHERE status = 'pending'
		), processing AS (
			SELECT
				count(*) AS processing_count,
				min(created_at) AS processing_oldest,
				count(*) FILTER (WHERE processing_started_at <= $2) AS stuck
			FROM webhook_events
			WHERE status = 'processing'
		), failed AS (
			SELECT count(*) AS failed_count, min(created_at) AS failed_oldest
			FROM webhook_events
			WHERE status = 'failed'
		)
		SELECT
			pending_count, pending_oldest, retry_count, retry_oldest,
			processing_count, processing_oldest, failed_count, failed_oldest,
			oldest_ready, stuck
		FROM pending CROSS JOIN processing CROSS JOIN failed
	`, now, staleBefore)
}

func (s *Store) queueStats(ctx context.Context, query string, now, staleBefore time.Time) (QueueStats, error) {
	var (
		pendingCount, retryCount, processingCount, failedCount int64
		pendingOldest, retryOldest, processingOldest           sql.NullTime
		failedOldest, oldestReady                              sql.NullTime
		stuck                                                  int64
	)
	err := s.queryRow(ctx, query, now, staleBefore).Scan(
		&pendingCount, &pendingOldest,
		&retryCount, &retryOldest,
		&processingCount, &processingOldest,
		&failedCount, &failedOldest,
		&oldestReady, &stuck,
	)
	if err != nil {
		return QueueStats{}, err
	}
	return QueueStats{
		States: []QueueStateStats{
			{State: QueueStatePending, Count: pendingCount, OldestCreatedAt: nullTimePointer(pendingOldest)},
			{State: QueueStateRetry, Count: retryCount, OldestCreatedAt: nullTimePointer(retryOldest)},
			{State: QueueStateProcessing, Count: processingCount, OldestCreatedAt: nullTimePointer(processingOldest)},
			{State: QueueStateFailed, Count: failedCount, OldestCreatedAt: nullTimePointer(failedOldest)},
		},
		OldestReadyAt: nullTimePointer(oldestReady),
		Stuck:         stuck,
	}, nil
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
