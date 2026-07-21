package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// runPageCommand executes one automation command against a page target,
// mirroring the aw ui.* design: snapshot returns a pruned tree with refs,
// click/fill take a ref (preferred) or CSS selector, screenshot returns a PNG
// data URI to the tool adapter, which decides how much of it is safe to expose
// to the model. Page text is never logged here — results go to the caller only.
func runPageCommand(ctx context.Context, port int, command string, params map[string]any) (any, error) {
	page, err := pickPage(ctx, port, params)
	if err != nil {
		return nil, err
	}
	s, err := dialPage(ctx, page.WebSocketDebuggerURL)
	if err != nil {
		return nil, err
	}
	defer s.close()

	switch command {
	case "navigate":
		return navigate(ctx, s, params)
	case "navigationTarget", "navigationtarget":
		return navigationTarget(ctx, s, params)
	case "currentURL", "currenturl":
		return currentURL(ctx, s)
	case "snapshot":
		return snapshot(ctx, s, params)
	case "click":
		return click(ctx, s, params)
	case "fill":
		return fill(ctx, s, params)
	case "screenshot":
		return screenshot(ctx, s)
	case "cdp":
		return cdp(ctx, s, params)
	default:
		return nil, fmt.Errorf("unknown browser command %q", command)
	}
}

// closeTabs closes page targets by id, or every tab whose URL or title
// contains the given substring (case-insensitive) — "close all device
// activation tabs" in one call. Returns how many were closed and which.
func closeTabs(ctx context.Context, port int, params map[string]any) (any, error) {
	tab, _ := params["tab"].(string)
	urlMatch := strings.ToLower(strings.TrimSpace(asString(params["url"])))
	titleMatch := strings.ToLower(strings.TrimSpace(asString(params["title"])))
	if tab == "" && urlMatch == "" && titleMatch == "" {
		return nil, fmt.Errorf("close_tab needs a tab id, or a url or title substring to match")
	}
	targets, err := listTargets(ctx, port)
	if err != nil {
		return nil, err
	}
	wsURL, err := browserWSURL(ctx, port)
	if err != nil {
		return nil, err
	}
	s, err := dialPage(ctx, wsURL)
	if err != nil {
		return nil, err
	}
	defer s.close()
	closed := make([]map[string]any, 0)
	for _, t := range targets {
		if t.Type != "page" {
			continue
		}
		match := (tab != "" && t.ID == tab) ||
			(urlMatch != "" && strings.Contains(strings.ToLower(t.URL), urlMatch)) ||
			(titleMatch != "" && strings.Contains(strings.ToLower(t.Title), titleMatch))
		if !match {
			continue
		}
		if _, err := s.call(ctx, "Target.closeTarget", map[string]any{"targetId": t.ID}); err != nil {
			continue
		}
		closed = append(closed, map[string]any{"id": t.ID, "title": t.Title, "url": t.URL})
	}
	return map[string]any{"closed": len(closed), "tabs": closed}, nil
}

func newTab(ctx context.Context, port int, params map[string]any) (any, error) {
	targetURL := strings.TrimSpace(asString(params["url"]))
	if targetURL == "" {
		targetURL = "about:blank"
	}
	wsURL, err := browserWSURL(ctx, port)
	if err != nil {
		return nil, err
	}
	s, err := dialPage(ctx, wsURL)
	if err != nil {
		return nil, err
	}
	defer s.close()
	value, err := s.call(ctx, "Target.createTarget", map[string]any{"url": targetURL})
	if err != nil {
		return nil, err
	}
	var created struct {
		TargetID string `json:"targetId"`
	}
	if err := json.Unmarshal(value, &created); err != nil {
		return nil, fmt.Errorf("decode new tab target: %w", err)
	}
	if created.TargetID == "" {
		return nil, fmt.Errorf("browser did not return a new tab id")
	}

	deadline := time.Now().Add(10 * time.Second)
	var found target
	for time.Now().Before(deadline) {
		targets, err := listTargets(ctx, port)
		if err == nil {
			for _, t := range targets {
				if t.ID == created.TargetID {
					found = t
					if t.Type == "page" && t.WebSocketDebuggerURL != "" && (targetURL == "about:blank" || strings.TrimSpace(t.URL) != "") {
						return map[string]any{"created": true, "tab": t.ID, "url": t.URL, "title": t.Title}, nil
					}
					break
				}
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return map[string]any{"created": true, "tab": created.TargetID, "url": found.URL, "title": found.Title}, nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func navigationTarget(ctx context.Context, s *session, params map[string]any) (any, error) {
	ref, selector, err := targetArgs(params)
	if err != nil {
		return nil, err
	}
	value, err := s.evaluate(ctx, fmt.Sprintf("(%s)(%s, %s)", navigationTargetJS, jsString(ref), jsString(selector)))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, fmt.Errorf("decode navigation target: %w", err)
	}
	return result, nil
}

func currentURL(ctx context.Context, s *session) (any, error) {
	value, err := s.evaluate(ctx, fmt.Sprintf("(%s)()", currentURLJS))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, fmt.Errorf("decode current url: %w", err)
	}
	return result, nil
}

// pickPage selects the target tab: params["tab"] (id from browser.tabs) or
// the first open page.
func pickPage(ctx context.Context, port int, params map[string]any) (target, error) {
	targets, err := listTargets(ctx, port)
	if err != nil {
		return target{}, err
	}
	wanted, _ := params["tab"].(string)
	var first *target
	for i := range targets {
		t := &targets[i]
		if t.Type != "page" || t.WebSocketDebuggerURL == "" {
			continue
		}
		if wanted != "" && t.ID == wanted {
			return *t, nil
		}
		if first == nil {
			first = t
		}
	}
	if wanted != "" {
		return target{}, fmt.Errorf("tab %q not found (use browser.tabs to list)", wanted)
	}
	if first == nil {
		return target{}, fmt.Errorf("the browser has no open page")
	}
	return *first, nil
}

func navigate(ctx context.Context, s *session, params map[string]any) (any, error) {
	url, _ := params["url"].(string)
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}
	if _, err := s.call(ctx, "Page.navigate", map[string]any{"url": url}); err != nil {
		return nil, err
	}
	// Best-effort load wait: poll readyState; navigation may swap documents,
	// so evaluate errors during the swap are retried until the deadline.
	deadline := time.Now().Add(10 * time.Second)
	state := ""
	for time.Now().Before(deadline) {
		value, err := s.evaluate(ctx, `document.readyState`)
		if err == nil {
			_ = json.Unmarshal(value, &state)
			if state == "complete" || state == "interactive" {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	title, _ := s.evaluate(ctx, `document.title`)
	var titleText string
	_ = json.Unmarshal(title, &titleText)
	return map[string]any{"navigated": true, "url": url, "title": titleText, "readyState": state}, nil
}

func snapshot(ctx context.Context, s *session, params map[string]any) (any, error) {
	maxNodes := 500
	if value, ok := params["max"].(float64); ok && value > 0 {
		maxNodes = int(value)
	}
	value, err := s.evaluate(ctx, fmt.Sprintf("(%s)(%d)", snapshotJS, maxNodes))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	return result, nil
}

func click(ctx context.Context, s *session, params map[string]any) (any, error) {
	ref, selector, err := targetArgs(params)
	if err != nil {
		return nil, err
	}
	value, err := s.evaluate(ctx, fmt.Sprintf("(%s)(%s, %s)", clickJS, jsString(ref), jsString(selector)))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, fmt.Errorf("decode click result: %w", err)
	}
	return result, nil
}

func fill(ctx context.Context, s *session, params map[string]any) (any, error) {
	ref, selector, err := targetArgs(params)
	if err != nil {
		return nil, err
	}
	text, _ := params["value"].(string)
	value, err := s.evaluate(ctx, fmt.Sprintf("(%s)(%s, %s, %s)", fillJS, jsString(ref), jsString(selector), jsString(text)))
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, fmt.Errorf("decode fill result: %w", err)
	}
	return result, nil
}

func screenshot(ctx context.Context, s *session) (any, error) {
	result, err := s.call(ctx, "Page.captureScreenshot", map[string]any{"format": "png"})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	return map[string]any{"dataUri": "data:image/png;base64," + parsed.Data}, nil
}

func cdp(ctx context.Context, s *session, params map[string]any) (any, error) {
	method, _ := params["method"].(string)
	method = strings.TrimSpace(method)
	if method == "" {
		return nil, fmt.Errorf("method is required")
	}
	rawParams, _ := params["params"].(map[string]any)
	result, err := s.call(ctx, method, rawParams)
	if err != nil {
		return nil, err
	}
	var decoded any
	if err := json.Unmarshal(result, &decoded); err != nil {
		return nil, fmt.Errorf("decode cdp result: %w", err)
	}
	return decoded, nil
}

func targetArgs(params map[string]any) (string, string, error) {
	ref, _ := params["ref"].(string)
	selector, _ := params["selector"].(string)
	if strings.TrimSpace(ref) == "" && strings.TrimSpace(selector) == "" {
		return "", "", fmt.Errorf("ref (from browser.snapshot) or selector is required")
	}
	return strings.TrimSpace(ref), strings.TrimSpace(selector), nil
}

func jsString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
