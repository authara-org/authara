package response

import (
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session/roles"
)

type User struct {
	ID        string       `json:"id"`
	Email     string       `json:"email"`
	Username  string       `json:"username"`
	Disabled  bool         `json:"disabled"`
	CreatedAt time.Time    `json:"created_at"`
	Roles     []roles.Role `json:"roles"`
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
