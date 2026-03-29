package accesspolicy

import "context"

type NoopEmailAccessPolicy struct{}

func (NoopEmailAccessPolicy) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	return true, nil
}
