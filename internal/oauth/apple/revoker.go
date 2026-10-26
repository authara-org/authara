package apple

import (
	"context"
	"errors"
)

type tokenRevoker interface {
	Revoke(context.Context, string) error
}

type fallbackRevoker struct {
	revokers []tokenRevoker
}

func NewFallbackRevoker(revokers ...tokenRevoker) tokenRevoker {
	configured := make([]tokenRevoker, 0, len(revokers))
	for _, revoker := range revokers {
		if revoker != nil {
			configured = append(configured, revoker)
		}
	}
	return fallbackRevoker{revokers: configured}
}

func (r fallbackRevoker) Revoke(ctx context.Context, token string) error {
	var errs []error
	for _, revoker := range r.revokers {
		if err := revoker.Revoke(ctx, token); err == nil {
			return nil
		} else {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
