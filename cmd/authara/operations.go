package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/bootstrap"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/validation"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/schema"
	storetx "github.com/authara-org/authara/internal/store/tx"
	"github.com/google/uuid"
)

const operationalCommandUsage = "usage: authara <operator|admin> <grant|revoke> --email user@example.com | authara allowlist <add|remove> --email user@example.com"

var errLastActiveAdmin = errors.New("cannot revoke the last active admin")

type operationalCommand struct {
	target string
	action string
	email  string
}

type operationalCommandExecutor func(context.Context, operationalCommand) (string, error)

type operationalStore interface {
	GetUserByEmail(context.Context, string) (domain.User, error)
	AddUserPlatformRoleByName(context.Context, uuid.UUID, string) error
	RemoveUserPlatformRoleByName(context.Context, uuid.UUID, string) error
	UserHasPlatformRole(context.Context, uuid.UUID, string) (bool, error)
	LockPlatformRoleByName(context.Context, string) error
	CountActiveUsersWithRole(context.Context, string) (int, error)
	RevokeAllSessionsForUser(context.Context, uuid.UUID, time.Time) error
	EnsureAllowedEmail(context.Context, string) error
	DeleteAllowedEmail(context.Context, string) error
}

type transactionRunner interface {
	WithTransaction(context.Context, func(context.Context) error) error
}

type userAccessRevoker interface {
	RevokeUser(context.Context, uuid.UUID, time.Time) error
}

type operationalDependencies struct {
	store       operationalStore
	tx          transactionRunner
	revocations userAccessRevoker
	now         func() time.Time
}

func isOperationalCommand(name string) bool {
	switch name {
	case "operator", "admin", "allowlist":
		return true
	default:
		return false
	}
}

func runOperationalCommand(
	ctx context.Context,
	args []string,
	out io.Writer,
	execute operationalCommandExecutor,
) error {
	command, err := parseOperationalCommand(args)
	if err != nil {
		return err
	}

	message, err := execute(ctx, command)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, message)
	return err
}

func parseOperationalCommand(args []string) (operationalCommand, error) {
	if len(args) < 2 {
		return operationalCommand{}, errors.New(operationalCommandUsage)
	}

	target, action := args[0], args[1]
	switch target {
	case "operator", "admin":
		if action != "grant" && action != "revoke" {
			return operationalCommand{}, errors.New(operationalCommandUsage)
		}
	case "allowlist":
		if action != "add" && action != "remove" {
			return operationalCommand{}, errors.New(operationalCommandUsage)
		}
	default:
		return operationalCommand{}, errors.New(operationalCommandUsage)
	}

	flags := flag.NewFlagSet(target+" "+action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	email := flags.String("email", "", "email address")
	if err := flags.Parse(args[2:]); err != nil {
		return operationalCommand{}, fmt.Errorf("%s: %w", operationalCommandUsage, err)
	}
	if flags.NArg() != 0 {
		return operationalCommand{}, errors.New(operationalCommandUsage)
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(*email))
	if !validation.IsValidEmail(normalizedEmail) {
		return operationalCommand{}, errors.New(operationalCommandUsage)
	}

	return operationalCommand{
		target: target,
		action: action,
		email:  normalizedEmail,
	}, nil
}

func executeOperationalCommandFromEnvironment(ctx context.Context, command operationalCommand) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	st, err := bootstrap.NewStore(cfg)
	if err != nil {
		return "", fmt.Errorf("connect database: %w", err)
	}
	defer func() {
		_ = st.Close()
	}()

	if err := bootstrap.CheckSchemaVersion(st, schema.RequiredSchemaVersion); err != nil {
		return "", err
	}

	deps := operationalDependencies{
		store: st,
		tx:    storetx.New(st),
		now:   time.Now,
	}

	if isRoleRevocation(command) {
		ca, err := bootstrap.NewCache(cfg)
		if err != nil {
			return "", fmt.Errorf("connect cache: %w", err)
		}
		defer func() {
			_ = ca.Close()
		}()
		deps.revocations = token.NewAccessTokenRevocations(ca, cfg.Token.AccessTokenTTL)
	}

	return executeOperationalCommand(ctx, deps, command)
}

func executeOperationalCommand(ctx context.Context, deps operationalDependencies, command operationalCommand) (string, error) {
	switch command.target {
	case "operator":
		return executeRoleCommand(ctx, deps, command, roles.DBOperatorRoleName)
	case "admin":
		return executeRoleCommand(ctx, deps, command, roles.DBAdminRoleName)
	case "allowlist":
		return executeAllowlistCommand(ctx, deps.store, command)
	default:
		return "", errors.New(operationalCommandUsage)
	}
}

func executeRoleCommand(
	ctx context.Context,
	deps operationalDependencies,
	command operationalCommand,
	roleName string,
) (string, error) {
	if command.action == "grant" {
		user, err := findOperationalUser(ctx, deps.store, command.email)
		if err != nil {
			return "", err
		}
		if err := deps.store.AddUserPlatformRoleByName(ctx, user.ID, roleName); err != nil {
			return "", fmt.Errorf("grant %s role: %w", command.target, err)
		}
		return fmt.Sprintf("%s role granted to %s (%s)", command.target, user.Email, user.ID), nil
	}

	if deps.tx == nil || deps.revocations == nil {
		return "", errors.New("role revocation dependencies are not configured")
	}

	now := deps.now
	if now == nil {
		now = time.Now
	}
	revokedAt := now()
	var user domain.User
	err := deps.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if roleName == roles.DBAdminRoleName {
			if err := deps.store.LockPlatformRoleByName(txCtx, roleName); err != nil {
				return err
			}
		}

		var err error
		user, err = findOperationalUser(txCtx, deps.store, command.email)
		if err != nil {
			return err
		}

		hasRole, err := deps.store.UserHasPlatformRole(txCtx, user.ID, roleName)
		if err != nil {
			return err
		}
		if roleName == roles.DBAdminRoleName && hasRole && user.DisabledAt == nil {
			activeAdmins, err := deps.store.CountActiveUsersWithRole(txCtx, roleName)
			if err != nil {
				return err
			}
			if activeAdmins <= 1 {
				return errLastActiveAdmin
			}
		}

		if err := deps.store.RemoveUserPlatformRoleByName(txCtx, user.ID, roleName); err != nil {
			return err
		}
		return deps.store.RevokeAllSessionsForUser(txCtx, user.ID, revokedAt)
	})
	if err != nil {
		return "", fmt.Errorf("revoke %s role: %w", command.target, err)
	}

	if err := deps.revocations.RevokeUser(ctx, user.ID, revokedAt); err != nil {
		return "", fmt.Errorf("revoke %s access tokens: %w", command.target, err)
	}
	return fmt.Sprintf("%s role revoked from %s (%s)", command.target, user.Email, user.ID), nil
}

func executeAllowlistCommand(ctx context.Context, st operationalStore, command operationalCommand) (string, error) {
	if command.action == "add" {
		if err := st.EnsureAllowedEmail(ctx, command.email); err != nil {
			return "", fmt.Errorf("add allowlist email: %w", err)
		}
		return fmt.Sprintf("%s added to the allowlist", command.email), nil
	}

	if err := st.DeleteAllowedEmail(ctx, command.email); err != nil && !errors.Is(err, store.ErrAllowedEmailNotFound) {
		return "", fmt.Errorf("remove allowlist email: %w", err)
	}
	return fmt.Sprintf("%s removed from the allowlist", command.email), nil
}

func findOperationalUser(ctx context.Context, st operationalStore, email string) (domain.User, error) {
	user, err := st.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return domain.User{}, fmt.Errorf("user %q does not exist: %w", email, err)
		}
		return domain.User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func isRoleRevocation(command operationalCommand) bool {
	return (command.target == "operator" || command.target == "admin") && command.action == "revoke"
}
