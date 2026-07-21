package domain

import (
	"fmt"
	"strings"
)

// Agent Browser module ids (two thin modules over one shared CDP core).
const (
	BrowserModuleChrome = "browser-chrome"
	BrowserModuleEdge   = "browser-edge"
)

// Browser profile modes for BrowserStartOptions. There is no "personal"
// mode: Chrome/Edge 136+ ignore the remote-debugging port on the default
// user data dir, so attaching to the user's own profile is impossible — the
// agent browser always runs in its own isolated profile dir.
const (
	// BrowserProfileaw: the isolated aw-owned profile. Persistent but
	// separate from the user's. The default.
	BrowserProfileaw = "aw"
	// BrowserProfileInPrivate: the isolated aw profile in InPrivate/
	// Incognito mode — nothing persists.
	BrowserProfileInPrivate = "inprivate"
)

// ParseBrowserProfile normalizes a profile name; empty defaults to the
// isolated aw profile.
func ParseBrowserProfile(profile string) (string, error) {
	profile = strings.ToLower(strings.TrimSpace(profile))
	switch profile {
	case "":
		return BrowserProfileaw, nil
	case BrowserProfileaw, BrowserProfileInPrivate:
		return profile, nil
	default:
		return "", fmt.Errorf("unknown profile %q (use %s or %s)",
			profile, BrowserProfileaw, BrowserProfileInPrivate)
	}
}

// BrowserStartOptions configures one launch of a managed browser.
type BrowserStartOptions struct {
	Headless bool `json:"headless,omitempty"`
	// Profile is one of the BrowserProfile* modes; empty means aw.
	Profile string `json:"profile,omitempty"`
}

// BrowserStatus describes one managed browser instance.
type BrowserStatus struct {
	ID         string `json:"id"` // module id, e.g. "browser-chrome"
	Running    bool   `json:"running"`
	Headless   bool   `json:"headless,omitempty"`
	Port       int    `json:"port"`
	ProfileDir string `json:"profileDir"`
	Binary     string `json:"binary"`
	Notice     string `json:"notice,omitempty"`
}

// BrowserTab is one open page in a managed browser.
type BrowserTab struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}
