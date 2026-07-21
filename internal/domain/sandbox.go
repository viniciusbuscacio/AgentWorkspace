package domain

import "runtime"

// SandboxMode selects how the permissions sandbox fences the agent's
// filesystem and shell reach. Three modes: block_all ("Block all"),
// permit_list ("Balanced", the default) and permit_all ("Permit all").
// deny_list (default-allow with exceptions) existed and was removed as
// confusing; a vault still storing it fails safe to permit_list on load.
type SandboxMode string

const (
	// SandboxBlockAll disables the shell entirely; filesystem access is
	// limited to the agent workspace folder.
	SandboxBlockAll SandboxMode = "block_all"
	// SandboxPermitList is default-deny: only the workspace folder, the
	// built-in allowed folders and the user's allowed folders are reachable.
	SandboxPermitList SandboxMode = "permit_list"
	// SandboxPermitAll lifts path/shell restrictions; only the built-in
	// sensitive paths stay denied.
	SandboxPermitAll SandboxMode = "permit_all"
)

// ClampSandboxModeForUntrusted tightens a mode so a turn handling untrusted
// (tainted) external content can never run looser than permit_list. It ONLY
// restricts, never widens, and is applied transiently per turn — it never
// touches the user's persisted mode.
func ClampSandboxModeForUntrusted(mode SandboxMode) SandboxMode {
	if mode == SandboxPermitAll {
		return SandboxPermitList
	}
	// block_all and permit_list are already at least as restrictive.
	return mode
}

// SandboxModes lists the valid modes in UI order.
func SandboxModes() []SandboxMode {
	return []SandboxMode{SandboxBlockAll, SandboxPermitList, SandboxPermitAll}
}

// Valid reports whether the mode is one of the three known modes.
func (mode SandboxMode) Valid() bool {
	switch mode {
	case SandboxBlockAll, SandboxPermitList, SandboxPermitAll:
		return true
	}
	return false
}

// SandboxConfig is the user-controlled permissions configuration. It persists
// in the vault (never in config.json, which the agent could edit to widen its
// own fence) and is enforced in Go before every filesystem or shell operation.
type SandboxConfig struct {
	Mode SandboxMode `json:"mode"`
	// AllowedFolders are the user's extra reachable folders in permit_list.
	AllowedFolders []string `json:"allowedFolders"`
}

// DefaultSandboxConfig is the configuration used until the user changes it:
// permit_list with no extra folders.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		Mode:           SandboxPermitList,
		AllowedFolders: []string{},
	}
}

// sandboxBuiltinDeniedUnix applies in every mode, including permit_all (AW2
// list, minus AW2-specific entries; the aw data dir and vault are protected
// via the runtime policy, not this static list).
var sandboxBuiltinDeniedUnix = []string{
	"~/.ssh",
	"~/.gnupg",
	"~/.aws",
	"/etc/shadow",
	"/etc/passwd",
}

// sandboxBuiltinDeniedWindows keeps the same cross-platform secret folders
// plus Windows credential and shell-history locations. It intentionally stays
// small: the aw data dir, vault and user-configured Protected paths cover app
// state at runtime.
var sandboxBuiltinDeniedWindows = []string{
	"~/.ssh",
	"~/.gnupg",
	"~/.aws",
	`~/AppData/Roaming/Microsoft/Credentials`,
	`~/AppData/Local/Microsoft/Credentials`,
	`~/AppData/Roaming/Microsoft/Windows/PowerShell/PSReadLine`,
	`~/AppData/Roaming/Microsoft/UserSecrets`,
}

// sandboxBuiltinAllowedPermitList always passes in permit_list mode,
// alongside the workspace folder. (AW2 also allowed ~/.npm and
// ~/.node_modules — Node runtime legacy, dropped.)
var sandboxBuiltinAllowedPermitList = []string{
	"/tmp",
	"/var/tmp",
}

// SandboxBuiltinDenied returns the built-in always-denied paths: the checker
// enforces them, the prompt names them, the Permissions UI badges them.
func SandboxBuiltinDenied() []string {
	return sandboxBuiltinDeniedForGOOS(runtime.GOOS)
}

func sandboxBuiltinDeniedForGOOS(goos string) []string {
	if goos == "windows" {
		return append([]string(nil), sandboxBuiltinDeniedWindows...)
	}
	return append([]string(nil), sandboxBuiltinDeniedUnix...)
}

// SandboxBuiltinAllowedPermitList returns the built-in always-allowed
// permit_list folders.
func SandboxBuiltinAllowedPermitList() []string {
	return append([]string(nil), sandboxBuiltinAllowedPermitList...)
}
