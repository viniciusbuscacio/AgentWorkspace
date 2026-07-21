//go:build !awd_tray

package main

import "fmt"

// runTray (stub) — the systray companion is compiled only with -tags awd_tray so
// the default headless Linux-server build carries no systray CGO dependency. The
// tray is a separate user-session process that shows status and shortcuts; it
// never opens the vault (no second SQLite writer). See tray.go for the real one.
func runTray() error {
	return fmt.Errorf("awd tray was not built into this binary; build with -tags awd_tray (desktop session only)")
}
