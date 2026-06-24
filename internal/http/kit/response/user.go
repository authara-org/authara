package response

import (
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session/roles"
)

type User struct {
	ID           string        `json:"id"`
	Email        string        `json:"email"`
	Username     string        `json:"username"`
	Disabled     bool          `json:"disabled"`
	CreatedAt    time.Time     `json:"created_at"`
	Roles        []roles.Role  `json:"roles"`
	Organization *Organization `json:"organization,omitempty"`
}

type Organization struct {
	ID   string                  `json:"id"`
	Name string                  `json:"name"`
	Role domain.OrganizationRole `json:"role"`
}

func UserWithRoles(u domain.User, roles []roles.Role) User {
	return User{
		ID:        u.ID.String(),
		Email:     u.Email,
		Username:  u.Username,
		Disabled:  u.DisabledAt != nil,
		CreatedAt: u.CreatedAt,
		Roles:     roles,
	}
}

func UserWithRolesAndOrganization(u domain.User, roles []roles.Role, org domain.Organization, orgRole domain.OrganizationRole) User {
	out := UserWithRoles(u, roles)
	out.Organization = &Organization{
		ID:   org.ID.String(),
		Name: org.Name,
		Role: orgRole,
	}
	return out
}
