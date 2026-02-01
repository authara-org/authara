package roles

import "slices"

type Role string

const (
	AuthgateAdmin Role = "authgate:admin"
)

type Roles struct {
	Roles []Role
}

func (r *Roles) AddAdmin() {
	if !slices.Contains(r.Roles, AuthgateAdmin) {
		r.Roles = append(r.Roles, AuthgateAdmin)
	}
}

func (r *Roles) IsAdmin() bool {
	return slices.Contains(r.Roles, AuthgateAdmin)
}
