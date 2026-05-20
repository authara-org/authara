package admin

import "errors"

var (
	ErrSelfDisable              = errors.New("admin cannot disable themselves")
	ErrSelfRevokeAdmin          = errors.New("admin cannot remove their own admin role")
	ErrSelfRevokeSessions       = errors.New("admin cannot revoke all sessions for themselves")
	ErrLastAdmin                = errors.New("operation would leave no active admins")
	ErrAllowlistDisabled        = errors.New("allowlist feature disabled")
	ErrAllowedEmailAlreadyAdded = errors.New("allowed email already exists")
	ErrInvalidEmail             = errors.New("invalid email")
)

const (
	ReasonSelfDisable        = "You cannot disable your own account."
	ReasonSelfRevokeAdmin    = "You cannot remove your own admin role."
	ReasonSelfRevokeSessions = "You cannot revoke all sessions for your own account from here."
	ReasonLastAdmin          = "You cannot remove the last admin."
)
