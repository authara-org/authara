package operations

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/authara-org/authara/internal/http/kit/validation"
	"github.com/authara-org/authara/internal/identity"
)

const operationalCommandUsage = "usage: authara <operator|admin> <grant|revoke> --email user@example.com | authara allowlist <add|remove> --email user@example.com"

type operationalCommand struct {
	target string
	action string
	email  string
}

type operationalCommandExecutor func(context.Context, operationalCommand) (string, error)

func IsCommand(name string) bool {
	switch name {
	case "operator", "admin", "allowlist":
		return true
	default:
		return false
	}
}

func Run(ctx context.Context, args []string, out io.Writer) error {
	return runOperationalCommand(ctx, args, out, executeOperationalCommandFromEnvironment)
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

	normalizedEmail := identity.CanonicalEmail(*email)
	if !validation.IsValidEmail(normalizedEmail) {
		return operationalCommand{}, errors.New(operationalCommandUsage)
	}

	return operationalCommand{
		target: target,
		action: action,
		email:  normalizedEmail,
	}, nil
}
