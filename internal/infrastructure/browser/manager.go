// Package browser manages real Chrome/Edge instances over the Chrome DevTools
// Protocol for the Agent Browser workspace modules. One shared CDP core, two
// thin module configurations. The default instance runs with a dedicated
// aw-owned profile directory and debugging port; the user's real profile is
// explicit opt-in.
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"aw/internal/domain"
)

// Overrides customizes binary paths and CDP ports (zero values use defaults).
type Overrides struct {
	ChromePath string
	EdgePath   string
	ChromePort int
	EdgePort   int
}

// Manager implements ports.BrowserAutomation over local browser processes.
type Manager struct {
	mu        sync.Mutex
	baseDir   string // aw data dir; profiles live under <baseDir>/browser-profiles/<id>
	overrides Overrides
	running   map[string]*instance
}

type instance struct {
	cfg      launchConfig
	cmd      *exec.Cmd
	headless bool
}

type launchConfig struct {
	id     string
	binary string
	port   int
	dir    string
}

const (
	defaultChromePort = 9322
	defaultEdgePort   = 9323
	startupTimeout    = 20 * time.Second
)

func NewManager(baseDir string, overrides Overrides) *Manager {
	return &Manager{
		baseDir:   baseDir,
		overrides: overrides,
		running:   map[string]*instance{},
	}
}

func (m *Manager) ConfigureExecutable(id string, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch strings.TrimSpace(id) {
	case domain.BrowserModuleChrome:
		m.overrides.ChromePath = strings.TrimSpace(path)
	case domain.BrowserModuleEdge:
		m.overrides.EdgePath = strings.TrimSpace(path)
	default:
		return fmt.Errorf("unknown browser %q (use %s or %s)", id, domain.BrowserModuleChrome, domain.BrowserModuleEdge)
	}
	return nil
}

func (m *Manager) DefaultExecutable(id string) (string, error) {
	switch strings.TrimSpace(id) {
	case domain.BrowserModuleChrome:
		return defaultChromePath(), nil
	case domain.BrowserModuleEdge:
		return defaultEdgePath(), nil
	default:
		return "", fmt.Errorf("unknown browser %q (use %s or %s)", id, domain.BrowserModuleChrome, domain.BrowserModuleEdge)
	}
}

// Start launches the browser if it is not already running and waits for the
// CDP endpoint to come up. The browser always runs in an isolated aw-owned
// profile dir: Chrome/Edge 136+ ignore --remote-debugging-port on the default
// user data dir, so the user's own profile cannot be attached to at all.
func (m *Manager) Start(ctx context.Context, id string, opts domain.BrowserStartOptions) (domain.BrowserStatus, error) {
	cfg, err := m.launchConfigFor(id)
	if err != nil {
		return domain.BrowserStatus{}, err
	}

	// The CDP endpoint is the source of truth for "running": on macOS the
	// launched process may relaunch the real browser and exit (Edge does),
	// so the child handle says nothing. Alive endpoint → already running.
	if cdpAlive(ctx, cfg.port) {
		return m.Status(ctx, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := os.Stat(cfg.binary); err != nil {
		return domain.BrowserStatus{}, fmt.Errorf("browser binary not found at %q (configure it in appconfig browser settings)", cfg.binary)
	}
	if err := os.MkdirAll(cfg.dir, 0o700); err != nil {
		return domain.BrowserStatus{}, fmt.Errorf("create browser profile dir: %w", err)
	}
	cmd := exec.Command(cfg.binary, launchArgs(cfg, opts)...)
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		return domain.BrowserStatus{}, fmt.Errorf("launch browser: %w", err)
	}
	m.running[id] = &instance{cfg: cfg, cmd: cmd, headless: opts.Headless}

	if err := waitForCDP(ctx, cfg.port); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		delete(m.running, id)
		return domain.BrowserStatus{}, fmt.Errorf("browser did not expose CDP on port %d: %w", cfg.port, err)
	}
	// The agent's browser must not write to disk via downloads — that would
	// bypass the permissions sandbox (spec Risk 7). Fail closed: no deny, no
	// browser.
	if err := denyDownloads(ctx, cfg.port); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		delete(m.running, id)
		return domain.BrowserStatus{}, fmt.Errorf("disable downloads on the agent browser: %w", err)
	}
	// Reap the launcher process in the background (it may exit immediately
	// after handing off to the real browser — that is fine).
	go func() { _ = cmd.Wait() }()
	status := m.statusForLocked(id, cfg)
	status.Running = true
	return status, nil
}

// denyDownloads turns off downloads browser-wide over the CDP browser
// websocket (Browser.setDownloadBehavior applies to pages opened later too).
func denyDownloads(ctx context.Context, port int) error {
	wsURL, err := browserWSURL(ctx, port)
	if err != nil {
		return err
	}
	s, err := dialPage(ctx, wsURL)
	if err != nil {
		return err
	}
	defer s.close()
	_, err = s.call(ctx, "Browser.setDownloadBehavior", map[string]any{"behavior": "deny"})
	return err
}

// Stop terminates the managed instance: a graceful Browser.close over the
// CDP browser websocket (works even when the real browser is not our child
// process), plus a kill of our launcher if it is still around. Stopping a
// non-running browser is a no-op.
func (m *Manager) Stop(ctx context.Context, id string) (domain.BrowserStatus, error) {
	cfg, err := m.launchConfigFor(id)
	if err != nil {
		return domain.BrowserStatus{}, err
	}
	if wsURL, err := browserWSURL(ctx, cfg.port); err == nil {
		if s, err := dialPage(ctx, wsURL); err == nil {
			_, _ = s.call(ctx, "Browser.close", nil)
			s.close()
		}
	}
	m.mu.Lock()
	if inst, ok := m.running[id]; ok {
		if processAlive(inst.cmd) {
			_ = inst.cmd.Process.Kill()
		}
		delete(m.running, id)
	}
	m.mu.Unlock()
	// Best effort: wait for the endpoint to actually go away so the returned
	// status (and an immediate UI refresh) does not still say running.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && cdpAlive(ctx, cfg.port) {
		time.Sleep(150 * time.Millisecond)
	}
	return m.Status(ctx, id)
}

// Status reports whether the managed instance is running and where. Running
// means the CDP endpoint answers — not that our launcher process is alive.
func (m *Manager) Status(ctx context.Context, id string) (domain.BrowserStatus, error) {
	cfg, err := m.launchConfigFor(id)
	if err != nil {
		return domain.BrowserStatus{}, err
	}
	m.mu.Lock()
	status := m.statusForLocked(id, cfg)
	m.mu.Unlock()
	status.Running = cdpAlive(ctx, cfg.port)
	if !status.Running {
		status.Headless = false
	}
	return status, nil
}

// Tabs lists the open pages of a running instance.
func (m *Manager) Tabs(ctx context.Context, id string) ([]domain.BrowserTab, error) {
	cfg, err := m.requireRunning(ctx, id)
	if err != nil {
		return nil, err
	}
	targets, err := listTargets(ctx, cfg.port)
	if err != nil {
		return nil, err
	}
	tabs := make([]domain.BrowserTab, 0, len(targets))
	for _, target := range targets {
		if target.Type != "page" {
			continue
		}
		tabs = append(tabs, domain.BrowserTab{ID: target.ID, Title: target.Title, URL: target.URL})
	}
	return tabs, nil
}

// Command runs one browser/page automation command. Page-level commands use
// the first open page or params["tab"]; browser-level commands such as
// new_tab and close_tab operate on the browser target.
func (m *Manager) Command(ctx context.Context, id string, command string, params map[string]any) (any, error) {
	cfg, err := m.requireRunning(ctx, id)
	if err != nil {
		return nil, err
	}
	if command == "new_tab" {
		return newTab(ctx, cfg.port, params)
	}
	if command == "close_tab" {
		return closeTabs(ctx, cfg.port, params)
	}
	return runPageCommand(ctx, cfg.port, command, params)
}

func (m *Manager) requireRunning(ctx context.Context, id string) (launchConfig, error) {
	cfg, err := m.launchConfigFor(id)
	if err != nil {
		return launchConfig{}, err
	}
	if !cdpAlive(ctx, cfg.port) {
		return launchConfig{}, fmt.Errorf("browser %q is not running (use browser.start first)", id)
	}
	return cfg, nil
}

// statusForLocked fills the static status fields; Running is decided by the
// caller (per the CDP endpoint).
func (m *Manager) statusForLocked(id string, cfg launchConfig) domain.BrowserStatus {
	status := domain.BrowserStatus{
		ID:         id,
		Port:       cfg.port,
		ProfileDir: cfg.dir,
		Binary:     cfg.binary,
	}
	if inst, ok := m.running[id]; ok {
		status.Headless = inst.headless
	}
	return status
}

func (m *Manager) launchConfigFor(id string) (launchConfig, error) {
	cfg := launchConfig{id: id, dir: filepath.Join(m.baseDir, "browser-profiles", id)}
	switch id {
	case domain.BrowserModuleChrome:
		cfg.binary = firstNonEmpty(m.overrides.ChromePath, defaultChromePath())
		cfg.port = firstNonZero(m.overrides.ChromePort, defaultChromePort)
	case domain.BrowserModuleEdge:
		cfg.binary = firstNonEmpty(m.overrides.EdgePath, defaultEdgePath())
		cfg.port = firstNonZero(m.overrides.EdgePort, defaultEdgePort)
	default:
		return launchConfig{}, fmt.Errorf("unknown browser %q (use %s or %s)", id, domain.BrowserModuleChrome, domain.BrowserModuleEdge)
	}
	return cfg, nil
}

func defaultChromePath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	case "windows":
		return `C:\Program Files\Google\Chrome\Application\chrome.exe`
	default:
		return "/usr/bin/google-chrome"
	}
}

func defaultEdgePath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"
	case "windows":
		return `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	default:
		return "/usr/bin/microsoft-edge"
	}
}

// launchArgs builds the browser command line: always the isolated aw profile
// dir; inprivate additionally runs InPrivate (Edge) / Incognito (Chrome) so
// nothing persists.
func launchArgs(cfg launchConfig, opts domain.BrowserStartOptions) []string {
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", cfg.port),
		"--no-first-run",
		"--no-default-browser-check",
		"--user-data-dir=" + cfg.dir,
	}
	if opts.Profile == domain.BrowserProfileInPrivate {
		if cfg.id == domain.BrowserModuleEdge {
			args = append(args, "--inprivate")
		} else {
			args = append(args, "--incognito")
		}
	}
	if opts.Headless {
		args = append(args, "--headless=new")
	}
	args = append(args, "about:blank")
	return args
}

func processAlive(cmd *exec.Cmd) bool {
	return cmd != nil && cmd.Process != nil && (cmd.ProcessState == nil || !cmd.ProcessState.Exited())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

// waitForCDP polls the DevTools HTTP endpoint until it answers.
func waitForCDP(ctx context.Context, port int) error {
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if cdpAlive(ctx, port) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", startupTimeout)
}

// cdpAlive reports whether the DevTools HTTP endpoint answers on the port.
// The port is aw-owned (dedicated per module), so an answering endpoint
// means our managed browser is up — whatever happened to the launcher PID.
func cdpAlive(ctx context.Context, port int) bool {
	reqCtx, cancel := context.WithTimeout(contextOrBackground(ctx), 800*time.Millisecond)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// browserWSURL returns the browser-level CDP websocket (for Browser.close).
func browserWSURL(ctx context.Context, port int) (string, error) {
	reqCtx, cancel := context.WithTimeout(contextOrBackground(ctx), 2*time.Second)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return "", err
	}
	if version.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("no browser websocket url")
	}
	return version.WebSocketDebuggerURL, nil
}

type target struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func listTargets(ctx context.Context, port int) ([]target, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	req, err := http.NewRequestWithContext(contextOrBackground(ctx), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query browser targets: %w", err)
	}
	defer resp.Body.Close()
	var targets []target
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, fmt.Errorf("decode browser targets: %w", err)
	}
	return targets, nil
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
