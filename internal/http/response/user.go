package response

import (
	"time"

	"github.com/alexlup06-authgate/authgate/internal/domain"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
}

func UserFromDomain(u domain.User) User {
	return User{
		ID:        u.ID.String(),
		Email:     u.Email,
		Username:  u.Username,
		Disabled:  u.DisabledAt != nil,
		CreatedAt: u.CreatedAt,
	}
}
