package tools

import (
	"context"
	"strings"
	"testing"

	"aw/internal/domain"
	"aw/internal/infrastructure/externaltaint"
)

func passwordsWorkspace(t *testing.T, added *[]string) *workspace {
	t.Helper()
	return &workspace{
		root:           t.TempDir(),
		control:        &fakeControl{},
		addedModulesFn: func() []string { return *added },
		autoApprove:    false,
		passwords: &PasswordsFuncs{
			List: func(_ context.Context) (any, error) {
				return []map[string]any{{"id": "pw-1", "name": "Email", "username": "u@x.test", "url": ""}}, nil
			},
			Get: func(_ context.Context, idOrName string) (any, error) {
				return map[string]any{"id": idOrName, "name": "Email", "password": "s3cret"}, nil
			},
		},
	}
}

func TestPasswordsActionsGatedByModuleAdded(t *testing.T) {
	added := []string{}
	ws := passwordsWorkspace(t, &added)
	if _, ok := ws.awRegistry()["passwords.list"]; ok {
		t.Fatalf("passwords.list must not register while the module is absent")
	}
	added = []string{"passwords"}
	reg := ws.awRegistry()
	if _, ok := reg["passwords.list"]; !ok {
		t.Fatalf("passwords.list missing with the module added")
	}
	if _, ok := reg["passwords.get"]; !ok {
		t.Fatalf("passwords.get missing with the module added")
	}
}

func TestPasswordsListCarriesNoValues(t *testing.T) {
	added := []string{"passwords"}
	ws := passwordsWorkspace(t, &added)
	out, err := ws.awRegistry()["passwords.list"](context.Background(), map[string]any{}, ws)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if strings.Contains(out, "s3cret") || strings.Contains(out, "password") {
		t.Fatalf("list output must never carry values: %s", out)
	}
}

func TestPasswordsGetReturnsValueOnCleanTurn(t *testing.T) {
	added := []string{"passwords"}
	ws := passwordsWorkspace(t, &added)
	out, err := ws.awRegistry()["passwords.get"](context.Background(), map[string]any{"id": "pw-1"}, ws)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(out, "s3cret") {
		t.Fatalf("get should return the credential value on a clean turn: %s", out)
	}
	if _, err := ws.awRegistry()["passwords.get"](context.Background(), map[string]any{}, ws); err == nil {
		t.Fatalf("get without id must error")
	}
}

func TestPasswordsGetIsRegatedOnTaintedTurn(t *testing.T) {
	added := []string{"passwords"}
	ws := passwordsWorkspace(t, &added)
	ws.taintStore = externaltaint.NewStore()
	ctx := context.Background()
	// Simulate the turn having read untrusted external content.
	ws.taintStore.Record(domain.ExternalTaintScope(ctx), domain.ExternalContentSafety{
		Untrusted:  true,
		SourceType: domain.ExternalSourceWeb,
		Origin:     "https://example.test",
		RiskLevel:  domain.ExternalRiskHigh,
		Suspicious: true,
	})
	// With no confirmer wired (autoApprove=false, confirm=nil) the re-gate
	// must deny the reveal rather than hand the secret over.
	if _, err := ws.awRegistry()["passwords.get"](ctx, map[string]any{"id": "pw-1"}, ws); err == nil {
		t.Fatalf("expected the reveal to be denied on a tainted turn without approval")
	}
}
