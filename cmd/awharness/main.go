package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	var cfg Config
	var once bool
	flag.StringVar(&cfg.DataDir, "data-dir", "", "isolated AW data directory")
	flag.StringVar(&cfg.Workspace, "workspace", "", "agent workspace directory")
	flag.StringVar(&cfg.Addr, "addr", DefaultAddr, "REST listen address")
	flag.StringVar(&cfg.Token, "token", DefaultToken, "REST bearer token")
	flag.StringVar(&cfg.Password, "password", "", "dev vault password")
	flag.StringVar(&cfg.Profile, "profile", "", "dev profile name")
	flag.StringVar(&cfg.Browser.ChromePath, "chrome", "", "custom Chrome executable")
	flag.StringVar(&cfg.Browser.EdgePath, "edge", "", "custom Edge executable")
	flag.IntVar(&cfg.Browser.ChromePort, "chrome-port", 0, "Chrome CDP port")
	flag.IntVar(&cfg.Browser.EdgePort, "edge-port", 0, "Edge CDP port")
	flag.BoolVar(&cfg.SelfDev, "permit-all", false, "use permit_all sandbox policy")
	flag.BoolVar(&cfg.Chat, "chat", false, "enable chat mode: unlock the REAL vault and expose POST /api/chat")
	flag.BoolVar(&once, "once", false, "start, print info, then shut down")
	flag.Parse()

	// In chat mode the harness unlocks the user's REAL vault. The password comes
	// from AW_VAULT_PASSWORD (preferred) or the -password flag.
	cfg.VaultPassword = os.Getenv("AW_VAULT_PASSWORD")
	if strings.TrimSpace(cfg.VaultPassword) == "" {
		cfg.VaultPassword = cfg.Password
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := Start(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "awharness: %v\n", err)
		os.Exit(1)
	}
	info, _ := json.MarshalIndent(server.Info(), "", "  ")
	fmt.Println(string(info))
	if once {
		server.Shutdown(context.Background())
		return
	}
	<-ctx.Done()
	server.Shutdown(context.Background())
}
