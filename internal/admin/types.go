package admin

import (
	"slices"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/google/uuid"
)

type Actor struct {
	UserID uuid.UUID
	Email  string
	Roles  roles.Roles
}

type RequestMeta struct {
	IP        string
	UserAgent string
}

type Page struct {
	Page int
	Size int
}

type DashboardStats struct {
	TotalUsers         int
	SignupsLast24Hours int
	DisabledUsers      int
	ActiveSessions     int
}

type UserSummary struct {
	ID                 uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DisabledAt         *time.Time
	Username           string
	Email              string
	Roles              []string
	AuthProviderCount  int
	ActiveSessionCount int
}

func (u UserSummary) Disabled() bool {
	return u.DisabledAt != nil
}

func (u UserSummary) HasRole(roleName string) bool {
	return slices.Contains(u.Roles, roleName)
}

type AuthProviderSummary struct {
	ID          uuid.UUID
	Provider    string
	CreatedAt   time.Time
	HasPassword bool
	HasOAuthID  bool
}

type PasskeySummary struct {
	ID             uuid.UUID
	Name           string
	CreatedAt      time.Time
	LastUsedAt     *time.Time
	CloneWarning   bool
	BackupEligible bool
	BackupState    bool
	DeviceLabel    string
	Transport      []string
}

type SessionSummary struct {
	ID               uuid.UUID
	CreatedAt        time.Time
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	UserAgent        string
	UserAgentSummary string
	Status           string
}

type ActionAvailability struct {
	Allowed bool
	Reason  string
}

type UserDetailActions struct {
	Disable           ActionAvailability
	Enable            ActionAvailability
	GrantAdmin        ActionAvailability
	RevokeAdmin       ActionAvailability
	RevokeAllSessions ActionAvailability
}

type UserDetail struct {
	User          UserSummary
	AuthProviders []AuthProviderSummary
	Passkeys      []PasskeySummary
	Sessions      []SessionSummary
	Actions       UserDetailActions
}

type AllowedEmailPage struct {
	Emails  []domain.AllowedEmail
	Query   string
	Page    int
	Size    int
	Total   int
	Message string
}

func (p AllowedEmailPage) HasPrevious() bool {
	return p.Page > 1
}

func (p AllowedEmailPage) HasNext() bool {
	return p.Page*p.Size < p.Total
}

type RecentFailures struct {
	EmailJobs  []domain.EmailJob
	Challenges []domain.Challenge
	Page       int
	Size       int
}

type AuditEventPage struct {
	Events []domain.AdminAuditEvent
	Page   int
	Size   int
}
