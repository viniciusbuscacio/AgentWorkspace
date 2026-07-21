//go:build awd_tray

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"fyne.io/systray"

	"aw/internal/application"
	"aw/internal/domain"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/webserver"
)

//go:embed trayicon.ico
var trayIcon []byte

// runTray is the desktop-session companion process (`awd tray`). It is a
// SEPARATE process from the session-0 service and deliberately never opens the
// vault or runs the App — no second SQLite writer. It only shows status and
// shortcuts: service up/down (from the OS service manager) plus vault
// locked/unlocked (from GET /healthz), an "open in browser" link, and
// start/stop (which may prompt for elevation). Compiled only with -tags awd_tray
// so the headless Linux-server build carries no systray CGO.
func runTray() error {
	systray.Run(trayOnReady, func() {})
	return nil
}

func trayOnReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("AW")
	systray.SetTooltip("Agent Workspace daemon")

	mStatus := systray.AddMenuItem("Checking…", "Daemon status")
	mStatus.Disable()
	systray.AddSeparator()
	mOpen := systray.AddMenuItem("Open in browser", "Open the web UI")
	mStart := systray.AddMenuItem("Start daemon", "Start the awd service")
	mStop := systray.AddMenuItem("Stop daemon", "Stop the awd service")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit tray", "Close this tray (does not stop the daemon)")

	url := webURL()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		refreshTray(mStatus, mStart, mStop, url)
		for range ticker.C {
			refreshTray(mStatus, mStart, mStop, url)
		}
	}()

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				_ = openBrowser(url)
			case <-mStart.ClickedCh:
				if svc, _, err := newService("", ""); err == nil {
					_ = svc.Start()
				}
			case <-mStop.ClickedCh:
				if svc, _, err := newService("", ""); err == nil {
					_ = svc.Stop()
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// refreshTray updates the status line and start/stop enablement. Service running
// state comes from the OS service manager; vault locked/unlocked from /healthz.
func refreshTray(mStatus, mStart, mStop *systray.MenuItem, url string) {
	running, locked := trayHealth(url)
	switch {
	case !running:
		mStatus.SetTitle("● Daemon stopped")
		mStart.Enable()
		mStop.Disable()
	case locked:
		mStatus.SetTitle("● Running — vault locked")
		mStart.Disable()
		mStop.Enable()
	default:
		mStatus.SetTitle("● Running — vault unlocked")
		mStart.Disable()
		mStop.Enable()
	}
}

func trayHealth(url string) (running, locked bool) {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "/healthz")
	if err != nil {
		return false, true
	}
	defer func() { _ = resp.Body.Close() }()
	// healthz reports {running, locked}; any 200 means the server is up.
	var body struct {
		Running bool `json:"running"`
		Locked  bool `json:"locked"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return true, body.Locked
}

func webURL() string {
	cfg := application.LoadWebServerConfig(appconfig.Store{})
	host := cfg.BindAddr
	if cfg.BindModeOrDefault() == domain.WebBindModeTailscale {
		if ip, err := webserver.DetectTailscaleIP(); err == nil {
			host = ip
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	scheme := "http"
	if cfg.TLSEnabledOrDefault() {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, cfg.PortOrDefault())
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
