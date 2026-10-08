package roles

import (
	"fmt"
	"slices"
)

type Role string

const (
	AutharaAdmin    Role = "authara:admin"
	AutharaOperator Role = "authara:operator"
)

const (
	DBAdminRoleName    = "admin"
	DBOperatorRoleName = "operator"
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
	r.add(AutharaAdmin)
}

func (r *Roles) AddOperator() {
	r.add(AutharaOperator)
}

func (r Roles) Has(role Role) bool {
	return slices.Contains(r.roles, role)
}

func (r Roles) HasAny(allowed ...Role) bool {
	for _, role := range allowed {
		if r.Has(role) {
			return true
		}
	}
	return false
}

func (r Roles) IsAdmin() bool {
	return r.Has(AutharaAdmin)
}

func (r Roles) IsOperator() bool {
	return r.Has(AutharaOperator)
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

func FromDBRoleNames(names []string) (Roles, error) {
	var r Roles

	for _, name := range names {
		switch name {
		case DBAdminRoleName:
			r.AddAdmin()
		case DBOperatorRoleName:
			r.AddOperator()
		default:
			return Roles{}, fmt.Errorf("unknown db role: %s", name)
		}
	}

	return r, nil
}

func validate(role Role) error {
	switch role {
	case AutharaAdmin, AutharaOperator:
		return nil
	default:
		return fmt.Errorf("invalid role: %s", role)
	}
}
