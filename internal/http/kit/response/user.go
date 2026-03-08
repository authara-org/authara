package response

import (
	"time"

	"github.com/authara-org/authara/internal/domain"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
	Roles     []string  `json:"roles"`
}

func UserWithRoles(u domain.User, roles []string) User {
	return User{
		ID:        u.ID.String(),
		Email:     u.Email,
		Username:  u.Username,
		Disabled:  u.DisabledAt != nil,
		CreatedAt: u.CreatedAt,
		Roles:     roles,
	}
}
