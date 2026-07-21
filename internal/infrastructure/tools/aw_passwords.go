package tools

import (
	"context"

	"aw/internal/domain"
)

// PasswordsFuncs are the injected Passwords-module operations (the composition
// root wires them to application use cases against the vault — this package
// never touches the vault directly). The module is shared-with-the-agent by
// design: everything the user stores here is meant for the agent to use.
type PasswordsFuncs struct {
	// List returns credential summaries (metadata only, never values).
	List func(ctx context.Context) (any, error)
	// Get returns one full credential, including the password value.
	Get func(ctx context.Context, idOrName string) (any, error)
}

// registerPasswordsActions adds the Passwords module's action group. Callers
// register it only when the module is added — module fencing is structural.
func registerPasswordsActions(reg map[string]AwActionHandler) {
	reg["passwords.list"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		result, err := w.passwords.List(ctx)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}

	// passwords.get reveals the credential value. On a turn tainted by
	// untrusted external content (web page, email, tool output) the reveal is
	// re-gated: injected instructions asking the agent to read a credential
	// are the classic exfiltration setup.
	reg["passwords.get"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		idOrName, err := awRequiredStringArg(args, "id")
		if err != nil {
			return "", err
		}
		if err := w.requireExternalActionGuard(ctx, "passwords.get", domain.ExternalActionReveal, map[string]any{
			"id": idOrName,
		}); err != nil {
			return "", err
		}
		result, err := w.passwords.Get(ctx, idOrName)
		if err != nil {
			return "", err
		}
		return awJSON(result)
	}
}
