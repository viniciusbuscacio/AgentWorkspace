package appcore

import (
	"context"
	"encoding/json"
	"fmt"

	"aw/internal/dto"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// uiCommandResult carries the frontend's reply to a ui:command round-trip.
type uiCommandResult struct {
	result json.RawMessage
	errMsg string
}

// uiAutomation implements tools.Options.UIAutomationFn: it emits a "ui:command"
// event to the frontend and blocks until the frontend replies via
// ResolveUICommand (or the run is canceled). This is the raw UI-automation
// bridge (snapshot, click, fill, screenshot) backing the aw ui.* actions, and
// it mirrors confirmToolAction's request/response plumbing.
func (a *App) uiAutomation(ctx context.Context, command string, params map[string]any) (out any, err error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("ui automation unavailable: no window")
	}
	// Last-resort guard: a UI-automation request (especially a screenshot) must
	// never crash the app — better to return an error than to take the process
	// down. This recovers any Go panic in the bridge into a clean error.
	defer func() {
		if r := recover(); r != nil {
			out = nil
			err = fmt.Errorf("ui automation %q failed: %v", command, r)
		}
	}()
	// Screenshots prefer a native WKWebView snapshot: unlike the frontend's
	// DOM->SVG capture it preserves backdrop-filter (the wallpaper "glass" /
	// background-visibility effect), matching what the user actually sees
	// (parity with AW2's Electron capturePage). It must NEVER take the app down
	// — a failed screenshot is recoverable, a crash is not — so this runs behind
	// safeNativeScreenshot and always falls back to the frontend round-trip.
	if command == "screenshot" {
		// The native capture reads the window's composited pixels, so make sure
		// any just-issued navigation (e.g. a click that switched to "Apps") has
		// actually painted before we grab the frame. Best-effort: ignore errors.
		_, _ = a.roundTripUICommand(ctx, "settle", nil)
		if shot, ok := a.safeNativeScreenshot(); ok {
			return shot, nil
		}
	}
	return a.roundTripUICommand(ctx, command, params)
}

// roundTripUICommand emits a "ui:command" event to the frontend and blocks until
// the frontend replies via ResolveUICommand (or the run is canceled).
func (a *App) roundTripUICommand(ctx context.Context, command string, params map[string]any) (any, error) {
	id, err := newConfirmationID()
	if err != nil {
		return nil, err
	}
	ch := make(chan uiCommandResult, 1)
	a.uiCmdMu.Lock()
	if a.uiCommands == nil {
		a.uiCommands = map[string]chan uiCommandResult{}
	}
	a.uiCommands[id] = ch
	a.uiCmdMu.Unlock()
	defer func() {
		a.uiCmdMu.Lock()
		delete(a.uiCommands, id)
		a.uiCmdMu.Unlock()
	}()

	if params == nil {
		params = map[string]any{}
	}
	wailsruntime.EventsEmit(a.ctx, "ui:command", map[string]any{
		"id":      id,
		"command": command,
		"params":  params,
	})

	select {
	case reply := <-ch:
		if reply.errMsg != "" {
			return nil, fmt.Errorf("%s", reply.errMsg)
		}
		var decoded any
		if len(reply.result) == 0 {
			return map[string]any{"success": true}, nil
		}
		if err := json.Unmarshal(reply.result, &decoded); err != nil {
			return nil, fmt.Errorf("ui command returned invalid JSON: %w", err)
		}
		return decoded, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ResolveUICommand is called by the frontend to deliver the result of a
// pending ui:command round-trip. resultJSON is the command's JSON output;
// errMsg, when non-empty, reports a frontend-side failure.
func (a *App) ResolveUICommand(id string, resultJSON string, errMsg string) dto.OperationResult {
	a.uiCmdMu.Lock()
	ch := a.uiCommands[id]
	a.uiCmdMu.Unlock()
	if ch == nil {
		return dto.OperationResult{Success: false, Error: "unknown or expired ui command"}
	}
	select {
	case ch <- uiCommandResult{result: json.RawMessage(resultJSON), errMsg: errMsg}:
	default:
	}
	return dto.OperationResult{Success: true}
}
