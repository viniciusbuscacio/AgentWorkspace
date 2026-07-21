// Command awd is the headless Agent Workspace daemon. It runs the same App as
// the desktop GUI (internal/appcore) without a window, serving the full UI to a
// remote browser over the shared webserver package (web access is always-on in
// daemon mode — serving the web is the daemon's reason to exist). It installs
// itself as an OS service (Windows SCM / systemd / launchd) via kardianos/service.
//
// Process/service management (install/start/stop) is CLI + the OS service
// manager — never the web (that would be circular). App management (unlock the
// vault, use AW) is the browser: the daemon boots with the vault locked and the
// login/setup screen already live on the Tailscale IP/port from config.json.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kardianos/service"

	"aw/internal/appcore"
	"aw/internal/application"
	"aw/internal/infrastructure/appconfig"
	"aw/internal/infrastructure/webserver"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "awd:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("awd", flag.ContinueOnError)
	dataDir := fs.String("data-dir", "", "explicit data directory (vault + config); required when running as a system account that cannot see the user profile")
	serviceUser := fs.String("user", "", "OS account to run the service as (install only); default is the installing user")
	if err := fs.Parse(args); err != nil {
		return err
	}
	command := strings.ToLower(strings.TrimSpace(fs.Arg(0)))

	// A data dir given on the CLI is the source of truth for both the vault and
	// config.json, shared with the GUI's resolution (aw_DATA_DIR).
	if *dataDir != "" {
		if err := os.Setenv("aw_DATA_DIR", *dataDir); err != nil {
			return err
		}
	}

	if command == "tray" {
		return runTray()
	}

	svc, prog, err := newService(*serviceUser, *dataDir)
	if err != nil {
		return err
	}

	switch command {
	case "", "run":
		// Foreground (or launched by the service manager): boot and block.
		return svc.Run()
	case "install":
		if err := preflightInstall(*dataDir); err != nil {
			return err
		}
		if err := service.Control(svc, "install"); err != nil {
			return err
		}
		fmt.Println("awd installed. Start it with: awd start")
		return nil
	case "uninstall", "start", "stop", "restart":
		return service.Control(svc, command)
	case "status":
		return printStatus(svc, prog)
	default:
		return fmt.Errorf("unknown command %q (use: run|install|uninstall|start|stop|restart|status|tray)", command)
	}
}

func newService(user, dataDir string) (service.Service, *program, error) {
	prog := &program{}
	args := []string{"run"}
	if dataDir != "" {
		args = []string{"--data-dir", dataDir, "run"}
	}
	cfg := &service.Config{
		Name:        "awd",
		DisplayName: "Agent Workspace Daemon",
		Description: "Headless Agent Workspace server (full UI over the Tailscale web interface).",
		Arguments:   args,
		UserName:    user,
	}
	svc, err := service.New(prog, cfg)
	if err != nil {
		return nil, nil, err
	}
	return svc, prog, nil
}

// program implements service.Interface. Start must not block (kardianos
// contract): it kicks off the daemon goroutine and returns.
type program struct {
	app    *appcore.App
	cancel context.CancelFunc
}

func (p *program) Start(_ service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.app = appcore.NewApp()
	if dist, err := loadAssets(); err == nil && dist != nil {
		p.app.SetWebAssets(dist)
	}
	go p.serve(ctx)
	return nil
}

func (p *program) serve(ctx context.Context) {
	// StartHeadless runs the auto-lock loop and (if enabled in config) the web
	// server, with a.ctx left nil so events route to the web hub.
	p.app.StartHeadless(ctx)
	// Web access is always-on in daemon mode regardless of the config toggle —
	// serving the web is the whole point of the daemon. A start error (e.g. no
	// Tailscale IP yet) is logged into web status; the daemon keeps running so
	// the operator can fix the network without a crash loop.
	status := p.app.StartWebServer()
	if status.Error != "" {
		fmt.Fprintf(os.Stderr, "awd: web server not started: %s\n", status.Error)
	} else {
		fmt.Printf("awd: web access at %s (vault is locked — open it in a browser to log in)\n", status.URL)
	}
	<-ctx.Done()
}

func (p *program) Stop(_ service.Service) error {
	if p.app != nil {
		// Lock the vault and stop servers before exit.
		p.app.LockVault()
		p.app.Shutdown(context.Background())
	}
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

// preflightInstall fails early on the easy-to-get-wrong service-account vs
// vault-path combination: a system account (LocalSystem/root) cannot see a vault
// in the user profile, so an explicit --data-dir in a service-accessible
// location is required.
func preflightInstall(dataDir string) error {
	if service.Interactive() && dataDir == "" {
		// Installed under the current (interactive) user: the user profile is
		// visible, so the default data dir is fine.
		return nil
	}
	if dataDir == "" {
		return fmt.Errorf("a system service account cannot read a vault in the user profile; pass --data-dir <path> to a location the service account can access")
	}
	return nil
}

func printStatus(svc service.Service, _ *program) error {
	st, err := svc.Status()
	if err != nil {
		// Not installed as a service: report config so the operator still sees
		// where the web login will be.
		cfg := application.LoadWebServerConfig(appconfig.Store{})
		ip, _ := webserver.DetectTailscaleIP()
		fmt.Printf("awd: not installed as a service. Web access would bind %s mode on port %d (tailnet IP: %s)\n",
			cfg.BindModeOrDefault(), cfg.PortOrDefault(), orNone(ip))
		return nil
	}
	switch st {
	case service.StatusRunning:
		fmt.Println("awd: running")
	case service.StatusStopped:
		fmt.Println("awd: stopped")
	default:
		fmt.Println("awd: unknown")
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "none detected"
	}
	return s
}
