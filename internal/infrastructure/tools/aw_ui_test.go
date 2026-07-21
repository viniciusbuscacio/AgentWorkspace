package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func uiWorkspace(t *testing.T) (*workspace, *[]string) {
	t.Helper()
	var calls []string
	ws, err := newWorkspace(Options{
		Root: t.TempDir(),
		UIAutomationFn: func(_ context.Context, command string, params map[string]any) (any, error) {
			data, _ := json.Marshal(params)
			calls = append(calls, command+" "+string(data))
			return map[string]any{"command": command, "params": params}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ws, &calls
}

func TestUIActionsAreRegisteredOnlyWithBackend(t *testing.T) {
	without, err := newWorkspace(Options{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := without.awRegistry()["ui.snapshot"]; ok {
		t.Fatal("ui.snapshot must not be registered without a UI automation backend")
	}
	with, _ := uiWorkspace(t)
	for _, action := range []string{"ui.snapshot", "ui.click", "ui.fill", "ui.screenshot"} {
		if _, ok := with.awRegistry()[action]; !ok {
			t.Fatalf("expected %s to be registered", action)
		}
	}
}

func TestUIClickRequiresRefOrSelector(t *testing.T) {
	ws, calls := uiWorkspace(t)
	handler := ws.awRegistry()["ui.click"]
	if _, err := handler(context.Background(), map[string]any{}, ws); err == nil {
		t.Fatal("expected an error when neither ref nor selector is given")
	}
	if len(*calls) != 0 {
		t.Fatal("expected no round-trip when validation fails")
	}
	if _, err := handler(context.Background(), map[string]any{"ref": "e7"}, ws); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0] != `click {"ref":"e7"}` {
		t.Fatalf("unexpected round-trip calls: %v", *calls)
	}
}

func TestUIClickAcceptsSelectorToo(t *testing.T) {
	ws, calls := uiWorkspace(t)
	handler := ws.awRegistry()["ui.click"]
	if _, err := handler(context.Background(), map[string]any{"selector": "#go"}, ws); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0] != `click {"selector":"#go"}` {
		t.Fatalf("unexpected round-trip calls: %v", *calls)
	}
}

func TestUIFillPassesRefAndValue(t *testing.T) {
	ws, calls := uiWorkspace(t)
	handler := ws.awRegistry()["ui.fill"]
	if _, err := handler(context.Background(), map[string]any{"ref": "e5", "value": "9305"}, ws); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0] != `fill {"ref":"e5","value":"9305"}` {
		t.Fatalf("unexpected fill round-trip: %v", *calls)
	}
}

func TestUIFillRequiresValue(t *testing.T) {
	ws, calls := uiWorkspace(t)
	handler := ws.awRegistry()["ui.fill"]
	// A missing "value" (e.g. a caller that misnamed it "text") must error
	// instead of silently clearing the field with a success result.
	if _, err := handler(context.Background(), map[string]any{"ref": "e5", "text": "oops"}, ws); err == nil {
		t.Fatalf("expected error when value is absent")
	}
	if len(*calls) != 0 {
		t.Fatalf("no fill must reach the UI without a value: %v", *calls)
	}
	// An explicitly empty value is a legitimate clear.
	if _, err := handler(context.Background(), map[string]any{"ref": "e5", "value": ""}, ws); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 {
		t.Fatalf("explicit empty value must fill: %v", *calls)
	}
}

func TestUISnapshotForwardsMax(t *testing.T) {
	ws, calls := uiWorkspace(t)
	handler := ws.awRegistry()["ui.snapshot"]
	if _, err := handler(context.Background(), map[string]any{"max": 10}, ws); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || (*calls)[0] != `snapshot {"max":10}` {
		t.Fatalf("unexpected snapshot round-trip: %v", *calls)
	}
}

func TestUIDescriptionDocumentsActions(t *testing.T) {
	desc := awToolDescription(Options{
		UIAutomationFn: func(context.Context, string, map[string]any) (any, error) { return nil, nil },
	})
	for _, want := range []string{"ui.snapshot", "ui.click", "ui.fill", "ui.screenshot"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("expected description to mention %s", want)
		}
	}
}

// TestUIDescriptionFramesSelfInspection guards the fix for the agent denying
// it can screenshot/AX-tree itself: the always-on description must advertise
// the self-aliases and make the "yourself" framing explicit.
func TestUIDescriptionFramesSelfInspection(t *testing.T) {
	desc := awToolDescription(Options{
		UIAutomationFn: func(context.Context, string, map[string]any) (any, error) { return nil, nil },
	})
	for _, want := range []string{"app.screenshot", "app.snapshot", "YOURSELF"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("expected description to frame self-inspection with %q", want)
		}
	}
}

// TestSelfAliasesRoundTripToOwnUI verifies app.screenshot / app.snapshot are
// registered with a UI backend and delegate to the same own-UI commands.
func TestSelfAliasesRoundTripToOwnUI(t *testing.T) {
	ws, calls := uiWorkspace(t)
	reg := ws.awRegistry()
	for _, action := range []string{"app.screenshot", "app.snapshot"} {
		if _, ok := reg[action]; !ok {
			t.Fatalf("expected %s to be registered with a UI backend", action)
		}
	}
	if _, err := reg["app.screenshot"](context.Background(), map[string]any{}, ws); err != nil {
		t.Fatal(err)
	}
	if _, err := reg["app.snapshot"](context.Background(), map[string]any{"max": 10}, ws); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 || (*calls)[0] != `screenshot {}` || (*calls)[1] != `snapshot {"max":10}` {
		t.Fatalf("self-aliases must round-trip to own UI screenshot/snapshot: %v", *calls)
	}
}

// TestSelfAliasesRequireBackend keeps the aliases fail-closed: no UI backend,
// no app.screenshot / app.snapshot.
func TestSelfAliasesRequireBackend(t *testing.T) {
	without, err := newWorkspace(Options{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	reg := without.awRegistry()
	for _, action := range []string{"app.screenshot", "app.snapshot"} {
		if _, ok := reg[action]; ok {
			t.Fatalf("%s must not be registered without a UI automation backend", action)
		}
	}
}

func TestScreenshotSideChannelsImageToActiveChat(t *testing.T) {
	var shown []string
	ws, err := newWorkspace(Options{
		Root: t.TempDir(),
		UIAutomationFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return map[string]any{"dataUri": "data:image/jpeg;base64,QUJD", "width": float64(1280), "height": float64(700)}, nil
		},
		InlineImageFn: func(dataURI string, width, height int, caption string) bool {
			shown = append(shown, dataURI)
			if width != 1280 || height != 700 || caption == "" {
				t.Fatalf("unexpected inline-image args: %d %d %q", width, height, caption)
			}
			return true
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := ws.awRegistry()["app.screenshot"](context.Background(), map[string]any{}, ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(shown) != 1 || shown[0] != "data:image/jpeg;base64,QUJD" {
		t.Fatalf("expected the image to be side-channeled once, got %v", shown)
	}
	if strings.Contains(out, "QUJD") {
		t.Fatalf("model result must not carry the base64 blob: %q", out)
	}
	if !strings.Contains(out, "shown to the user") {
		t.Fatalf("model result should acknowledge the screenshot: %q", out)
	}
}

func TestScreenshotReturnsRawImageWithoutActiveChat(t *testing.T) {
	ws, err := newWorkspace(Options{
		Root: t.TempDir(),
		UIAutomationFn: func(_ context.Context, _ string, _ map[string]any) (any, error) {
			return map[string]any{"dataUri": "data:image/jpeg;base64,QUJD", "width": float64(1280), "height": float64(700)}, nil
		},
		InlineImageFn: func(string, int, int, string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := ws.awRegistry()["ui.screenshot"](context.Background(), map[string]any{}, ws)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "QUJD") {
		t.Fatalf("with no active chat the raw data URI must be returned: %q", out)
	}
}
