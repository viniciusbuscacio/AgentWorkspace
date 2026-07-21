package browser

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"aw/internal/domain"
)

// freePort reserves an ephemeral TCP port and frees it for the browser.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address type %T", listener.Addr())
	}
	_ = listener.Close()
	return addr.Port
}

func TestLaunchConfigDefaultsAndOverrides(t *testing.T) {
	m := NewManager("/data", Overrides{})
	cfg, err := m.launchConfigFor(domain.BrowserModuleChrome)
	if err != nil {
		t.Fatalf("launchConfigFor(chrome) error = %v", err)
	}
	if cfg.port != defaultChromePort || cfg.binary == "" {
		t.Fatalf("chrome defaults = %+v", cfg)
	}
	if !strings.Contains(cfg.dir, "browser-profiles") || !strings.Contains(cfg.dir, domain.BrowserModuleChrome) {
		t.Fatalf("profile dir must be aw-owned and per-module: %q", cfg.dir)
	}

	m = NewManager("/data", Overrides{EdgePath: "/custom/edge", EdgePort: 9999})
	cfg, err = m.launchConfigFor(domain.BrowserModuleEdge)
	if err != nil {
		t.Fatalf("launchConfigFor(edge) error = %v", err)
	}
	if cfg.binary != "/custom/edge" || cfg.port != 9999 {
		t.Fatalf("edge overrides not applied: %+v", cfg)
	}

	if _, err := m.launchConfigFor("browser-firefox"); err == nil {
		t.Fatal("unknown browser id should fail")
	}
}

func TestStatusAndCommandRequireRunningInstance(t *testing.T) {
	// Hermetic ports: Status probes the CDP endpoint, and the DEFAULT port
	// (9322) may be serving the user's real agent browser while tests run —
	// it was, once, and it turned the queue gates red. Never probe defaults.
	m := NewManager(t.TempDir(), Overrides{ChromePort: freePort(t), EdgePort: freePort(t)})
	status, err := m.Status(context.Background(), domain.BrowserModuleChrome)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Running {
		t.Fatalf("fresh manager should not report running: %+v", status)
	}
	if _, err := m.Command(context.Background(), domain.BrowserModuleChrome, "snapshot", nil); err == nil ||
		!strings.Contains(err.Error(), "not running") {
		t.Fatalf("Command on stopped browser should fail, got %v", err)
	}
	if _, err := m.Tabs(context.Background(), domain.BrowserModuleChrome); err == nil {
		t.Fatal("Tabs on stopped browser should fail")
	}
}

// TestStatusTrustsCDPEndpointNotTheLauncherPID is the Edge-on-macOS
// regression: the launched process can relaunch the real browser and exit,
// so "running" must mean "the CDP endpoint answers", never "our child
// process is alive". A stub DevTools endpoint with no child process at all
// must read as running.
func TestStatusTrustsCDPEndpointNotTheLauncherPID(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			_, _ = w.Write([]byte(`{"Browser":"stub"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer stub.Close()
	port, err := strconv.Atoi(strings.TrimPrefix(stub.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatalf("parse stub port from %q: %v", stub.URL, err)
	}

	m := NewManager(t.TempDir(), Overrides{EdgePort: port})
	status, err := m.Status(context.Background(), domain.BrowserModuleEdge)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Running {
		t.Fatalf("an answering CDP endpoint must report running (launcher PID is irrelevant): %+v", status)
	}
	// And page commands must be allowed to proceed past the running gate.
	if _, err := m.requireRunning(context.Background(), domain.BrowserModuleEdge); err != nil {
		t.Fatalf("requireRunning() error = %v", err)
	}
}

// Chrome/Edge 136+ ignore --remote-debugging-port on the default user data
// dir, so every launch must pin an isolated --user-data-dir.
func TestLaunchArgsAlwaysIsolateTheProfileDir(t *testing.T) {
	m := NewManager("/data", Overrides{})
	cfg, err := m.launchConfigFor(domain.BrowserModuleChrome)
	if err != nil {
		t.Fatalf("launchConfigFor(chrome) error = %v", err)
	}
	for _, opts := range []domain.BrowserStartOptions{{}, {Profile: domain.BrowserProfileInPrivate}, {Headless: true}} {
		joined := strings.Join(launchArgs(cfg, opts), " ")
		if !strings.Contains(joined, "--user-data-dir="+cfg.dir) {
			t.Fatalf("launch args must isolate the profile dir, got %q", joined)
		}
	}
}

// TestChromeEndToEnd drives a real headless Chrome when one is installed:
// start → navigate → snapshot (refs) → click → fill → screenshot → tabs →
// stop. Skipped when no Chrome binary is present.
func TestChromeEndToEnd(t *testing.T) {
	binary := defaultChromePath()
	if envBinary := os.Getenv("aw_CHROME"); envBinary != "" {
		binary = envBinary
	}
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("no Chrome binary at %q — skipping the CDP integration test", binary)
	}

	// A fresh free port per run: back-to-back test invocations must not race
	// a previous run's browser that is still shutting down on a fixed port.
	m := NewManager(t.TempDir(), Overrides{ChromePath: binary, ChromePort: freePort(t)})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	status, err := m.Start(ctx, domain.BrowserModuleChrome, domain.BrowserStartOptions{Headless: true})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _, _ = m.Stop(context.Background(), domain.BrowserModuleChrome) }()
	if !status.Running {
		t.Fatalf("status after start = %+v", status)
	}

	page := `<html><head><title>aw page</title></head><body>` +
		`<h1>aw test</h1>` +
		`<input id="name" placeholder="Your name">` +
		`<input id="password" type="password" aria-label="Password" value="supersecret">` +
		`<button id="copy"><span aria-hidden="true">content_copy</span>Copy</button>` +
		`<iframe title="Inner frame" srcdoc="<button id='inner' onclick=&quot;parent.document.title='frame-clicked'&quot;>Frame press</button>"></iframe>` +
		`<button onclick="document.title='clicked'">Press me</button>` +
		`</body></html>`
	pageURL := "data:text/html," + url.PathEscape(page)

	if _, err := m.Command(ctx, domain.BrowserModuleChrome, "navigate", map[string]any{"url": pageURL}); err != nil {
		t.Fatalf("navigate error = %v", err)
	}

	rawSnapshot, err := m.Command(ctx, domain.BrowserModuleChrome, "snapshot", nil)
	if err != nil {
		t.Fatalf("snapshot error = %v", err)
	}
	snapshotMap, ok := rawSnapshot.(map[string]any)
	if !ok {
		t.Fatalf("snapshot result type = %T", rawSnapshot)
	}
	tree, _ := snapshotMap["tree"].(string)
	if !strings.Contains(tree, "Press me") || !strings.Contains(tree, "[e") {
		t.Fatalf("snapshot tree missing button or refs:\n%s", tree)
	}
	if strings.Contains(tree, "content_copy") || !strings.Contains(tree, `button "Copy"`) {
		t.Fatalf("snapshot leaked decorative glyph or lost accessible button name:\n%s", tree)
	}
	if strings.Contains(tree, "supersecret") || !strings.Contains(tree, "••••••") {
		t.Fatalf("snapshot leaked password value or failed to mask it:\n%s", tree)
	}
	if !strings.Contains(tree, `iframe "Inner frame"`) || !strings.Contains(tree, `button "Frame press"`) {
		t.Fatalf("snapshot did not include same-origin iframe contents:\n%s", tree)
	}
	buttonRef := ""
	frameButtonRef := ""
	for _, line := range strings.Split(tree, "\n") {
		if strings.Contains(line, "Press me") {
			start := strings.Index(line, "[")
			end := strings.Index(line, "]")
			if start >= 0 && end > start {
				buttonRef = line[start+1 : end]
			}
		}
		if strings.Contains(line, "Frame press") {
			start := strings.Index(line, "[")
			end := strings.Index(line, "]")
			if start >= 0 && end > start {
				frameButtonRef = line[start+1 : end]
			}
		}
	}
	if buttonRef == "" {
		t.Fatalf("no ref found for the button:\n%s", tree)
	}
	if frameButtonRef == "" {
		t.Fatalf("no ref found for the iframe button:\n%s", tree)
	}

	if _, err := m.Command(ctx, domain.BrowserModuleChrome, "click", map[string]any{"ref": frameButtonRef}); err != nil {
		t.Fatalf("iframe click error = %v", err)
	}
	if _, err := m.Command(ctx, domain.BrowserModuleChrome, "click", map[string]any{"ref": buttonRef}); err != nil {
		t.Fatalf("click error = %v", err)
	}
	titleAfter, err := m.Command(ctx, domain.BrowserModuleChrome, "fill", map[string]any{"selector": "#name", "value": "Vinicius"})
	if err != nil {
		t.Fatalf("fill error = %v", err)
	}
	if filled, _ := titleAfter.(map[string]any)["filled"].(bool); !filled {
		t.Fatalf("fill result = %+v", titleAfter)
	}

	shot, err := m.Command(ctx, domain.BrowserModuleChrome, "screenshot", nil)
	if err != nil {
		t.Fatalf("screenshot error = %v", err)
	}
	if dataURI, _ := shot.(map[string]any)["dataUri"].(string); !strings.HasPrefix(dataURI, "data:image/png;base64,") {
		t.Fatalf("screenshot result = %v", shot)
	}

	cdpResult, err := m.Command(ctx, domain.BrowserModuleChrome, "cdp", map[string]any{
		"method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":    "document.title",
			"returnByValue": true,
		},
	})
	if err != nil {
		t.Fatalf("cdp error = %v", err)
	}
	cdpMap, ok := cdpResult.(map[string]any)
	if !ok {
		t.Fatalf("cdp result type = %T", cdpResult)
	}
	resultObj, ok := cdpMap["result"].(map[string]any)
	if !ok || resultObj["value"] == "" {
		t.Fatalf("cdp result = %+v", cdpResult)
	}

	tabs, err := m.Tabs(ctx, domain.BrowserModuleChrome)
	if err != nil || len(tabs) == 0 {
		t.Fatalf("Tabs() = %v, %v", tabs, err)
	}
	// The click set document.title to "clicked".
	if !strings.Contains(tabs[0].Title, "clicked") {
		t.Fatalf("click did not run in the page; tab title = %q", tabs[0].Title)
	}

	stopped, err := m.Stop(ctx, domain.BrowserModuleChrome)
	if err != nil || stopped.Running {
		t.Fatalf("Stop() = %+v, %v", stopped, err)
	}
}
