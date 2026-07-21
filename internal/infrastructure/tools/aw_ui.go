package tools

import (
	"context"
	"fmt"
	"strings"
)

// registerUIActions adds the raw UI-automation actions to the aw registry.
// They round-trip to the frontend through w.uiFn (a snapshot of the visible
// UI, clicking, filling fields and capturing a screenshot), letting an
// external agent drive any part of the app — not only the semantic app.*
// actions. Registered only when a UI automation backend is wired.
func registerUIActions(reg map[string]AwActionHandler) {
	reg["ui.snapshot"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		params := map[string]any{}
		if maxItems, has, err := awIntArg(args, "max"); err != nil {
			return "", err
		} else if has {
			params["max"] = maxItems
		}
		return w.runUICommand(ctx, "snapshot", params)
	}

	reg["ui.click"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		params, err := uiTargetParams(args)
		if err != nil {
			return "", err
		}
		return w.runUICommand(ctx, "click", params)
	}

	reg["ui.fill"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		params, err := uiTargetParams(args)
		if err != nil {
			return "", err
		}
		value, present, err := awStringArg(args, "value")
		if err != nil {
			return "", err
		}
		// A missing "value" must be an error, not an empty fill: a caller that
		// misnames the argument (e.g. "text") would otherwise silently CLEAR
		// the field and be told it succeeded.
		if !present {
			return "", fmt.Errorf(`"value" is required (pass "" to clear the field)`)
		}
		params["value"] = value
		return w.runUICommand(ctx, "fill", params)
	}

	reg["ui.screenshot"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		return w.runScreenshotCommand(ctx, "Screenshot")
	}

	// Self-aliases: the very same own-UI capture under app.* names, so the
	// agent reaches for them when asked to "take a screenshot of yourself" or
	// "read your own accessibility tree". These inspect THIS app's interface
	// (the chat the agent lives in), not the Browser module.
	reg["app.screenshot"] = func(ctx context.Context, _ map[string]any, w *workspace) (string, error) {
		return w.runScreenshotCommand(ctx, "Screenshot")
	}
	reg["app.snapshot"] = func(ctx context.Context, args map[string]any, w *workspace) (string, error) {
		params := map[string]any{}
		if maxItems, has, err := awIntArg(args, "max"); err != nil {
			return "", err
		} else if has {
			params["max"] = maxItems
		}
		return w.runUICommand(ctx, "snapshot", params)
	}
}

// uiTargetParams accepts a snapshot ref (preferred) or a CSS selector and
// requires at least one.
func uiTargetParams(args map[string]any) (map[string]any, error) {
	ref, _, err := awStringArg(args, "ref")
	if err != nil {
		return nil, err
	}
	selector, _, err := awStringArg(args, "selector")
	if err != nil {
		return nil, err
	}
	params := map[string]any{}
	if strings.TrimSpace(ref) != "" {
		params["ref"] = ref
	}
	if strings.TrimSpace(selector) != "" {
		params["selector"] = selector
	}
	if len(params) == 0 {
		return nil, fmt.Errorf("ref (from ui.snapshot) or selector is required")
	}
	return params, nil
}

func (w *workspace) runUICommand(ctx context.Context, command string, params map[string]any) (string, error) {
	if w.uiFn == nil {
		return "", fmt.Errorf("ui automation is not available")
	}
	result, err := w.uiFn(contextOrBackground(ctx), command, params)
	if err != nil {
		return "", err
	}
	return awJSON(result)
}

// runScreenshotCommand captures the app's own UI and, when a chat run is in
// flight, hands the picture to the user via the inline-image side-channel
// (mirroring AW2's workspace.screenshot). In that case the model gets only a
// short text acknowledgement — a full-resolution data URI in the tool result
// tokenizes to ~1M tokens and overflows the context window, and the model
// inspects the UI structurally through app.snapshot anyway. When there is no
// active chat (e.g. the REST self-test), the raw result (with the data URI) is
// returned unchanged.
func (w *workspace) runScreenshotCommand(ctx context.Context, caption string) (string, error) {
	if w.uiFn == nil {
		return "", fmt.Errorf("ui automation is not available")
	}
	result, err := w.uiFn(contextOrBackground(ctx), "screenshot", map[string]any{})
	if err != nil {
		return "", err
	}
	dataURI, width, height := screenshotResultFields(result)
	if w.inlineImageFn != nil && dataURI != "" {
		if shown := w.inlineImageFn(dataURI, width, height, caption); shown {
			return fmt.Sprintf("[system] Screenshot captured and shown to the user (%dx%d). Describe what is on screen naturally; the image itself is already visible to them.", width, height), nil
		}
	}
	return awJSON(result)
}

// screenshotResultFields pulls the data URI and pixel dimensions out of the
// frontend's screenshot reply ({dataUri,width,height}).
func screenshotResultFields(result any) (string, int, int) {
	m, ok := result.(map[string]any)
	if !ok {
		return "", 0, 0
	}
	dataURI, _ := m["dataUri"].(string)
	return dataURI, awAnyInt(m["width"]), awAnyInt(m["height"])
}

func awAnyInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
