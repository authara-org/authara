package roles

import (
	"fmt"
	"slices"
	"strings"
)

type Role string

const (
	AuthgateAdmin Role = "authgate:admin"
)

type Roles struct {
	roles []Role
}

func (r *Roles) List() []Role {
	return slices.Clone(r.roles)
}

func (r *Roles) add(role Role) {
	if !slices.Contains(r.roles, role) {
		r.roles = append(r.roles, role)
	}
}

func (r *Roles) AddAdmin() {
	if !slices.Contains(r.roles, AuthgateAdmin) {
		r.roles = append(r.roles, AuthgateAdmin)
	}
}

func (r *Roles) IsAdmin() bool {
	return slices.Contains(r.roles, AuthgateAdmin)
}

func FromClaims(claims []Role) (Roles, error) {
	var r Roles

	for _, role := range claims {
		if err := validate(role); err != nil {
			return Roles{}, err
		}

		r.add(role)
	}

	return r, nil
}

func validate(role Role) error {
	if !strings.HasPrefix(string(role), "authgate:") {
		return fmt.Errorf("invalid role namespace: %s", role)
	}
	return nil
}
