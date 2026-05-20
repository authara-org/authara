package admin

import (
	"context"
	"time"
)

func (s *Service) DashboardStats(ctx context.Context) (DashboardStats, error) {
	now := s.now()

	totalUsers, err := s.store.CountUsers(ctx)
	if err != nil {
		return DashboardStats{}, err
	}
	signupsLast24Hours, err := s.store.CountUsersCreatedSince(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return DashboardStats{}, err
	}
	disabledUsers, err := s.store.CountDisabledUsers(ctx)
	if err != nil {
		return DashboardStats{}, err
	}
	activeSessions, err := s.store.CountActiveSessions(ctx, now)
	if err != nil {
		return DashboardStats{}, err
	}

	return DashboardStats{
		TotalUsers:         totalUsers,
		SignupsLast24Hours: signupsLast24Hours,
		DisabledUsers:      disabledUsers,
		ActiveSessions:     activeSessions,
	}, nil
}
