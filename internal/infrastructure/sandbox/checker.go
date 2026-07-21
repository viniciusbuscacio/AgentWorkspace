// Package sandbox implements the permissions checker that fences the agent's
// filesystem and shell reach — a Go port of AW2's path-validator and
// sandbox-shell. The checker is pure decision logic (no vault or tools
// imports); the tools layer consults it before every filesystem or shell
// operation.
//
// ExtractPaths is a heuristic, not a POSIX shell, cmd.exe or PowerShell
// parser. It stops accidents and casual scope creep, not a determined
// adversary (obfuscated paths, $(...), env tricks, delayed expansion).
// Defense in depth: heuristic extraction + safe-command operator rejection +
// built-in denies at the fs layer + per-write confirmation.
package sandbox

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aw/internal/domain"
)

// Policy is the full enforcement context: the user's persisted configuration
// plus the runtime paths only the composition root knows.
type Policy struct {
	Config domain.SandboxConfig
	// WorkspaceRoot is the agent's workspace folder (tools Options.Root). It
	// is reachable in every mode — in block_all it is the only reachable
	// folder.
	WorkspaceRoot string
	// DataDir is the Agent Workspace application data directory. It is denied in every
	// mode, including permit_all, so the agent cannot edit config.json or the
	// vault and widen its own fence. The WorkspaceRoot carve-out is the only
	// exception (the default workspace lives inside the data dir).
	DataDir string
	// Protected are unconditionally denied paths with no carve-out (the
	// active vault directory, config.json).
	Protected []string
}

// The built-in lists live in the domain (the prompt names them, the UI
// badges them); the checker enforces them.
var (
	builtinAlwaysDenied      = domain.SandboxBuiltinDenied()
	builtinAllowedPermitList = domain.SandboxBuiltinAllowedPermitList()
)

// ExpandPath expands a leading ~ to the home directory and normalizes the
// path to an absolute, cleaned form.
func ExpandPath(path string) string {
	expanded := strings.TrimSpace(path)
	if expanded == "~" {
		expanded = homeDir()
	} else if strings.HasPrefix(expanded, "~/") {
		expanded = filepath.Join(homeDir(), expanded[2:])
	}
	if !filepath.IsAbs(expanded) {
		if abs, err := filepath.Abs(expanded); err == nil {
			expanded = abs
		}
	}
	return filepath.Clean(expanded)
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// normalizeForCompare puts a path in canonical slash form without a trailing
// separator so prefix checks behave consistently across platforms.
func normalizeForCompare(path string) string {
	normalized := filepath.ToSlash(filepath.Clean(path))
	if normalized != "/" {
		normalized = strings.TrimRight(normalized, "/")
	}
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}

// isInsideFolder reports whether filePath is inside folder (or is the folder
// itself). Both sides are tilde-expanded and normalized first.
func isInsideFolder(filePath string, folder string) bool {
	resolvedFile := normalizeForCompare(ExpandPath(filePath))
	resolvedFolder := normalizeForCompare(ExpandPath(folder))
	if resolvedFolder == "/" {
		return true
	}
	return resolvedFile == resolvedFolder || strings.HasPrefix(resolvedFile, resolvedFolder+"/")
}

func insideAny(filePath string, folders []string) bool {
	for _, folder := range folders {
		if strings.TrimSpace(folder) == "" {
			continue
		}
		if isInsideFolder(filePath, folder) {
			return true
		}
	}
	return false
}

// IsPathAllowed decides whether a path is reachable under the policy:
//
//   - built-in sensitive paths and Protected paths are blocked in every mode
//   - the data dir is blocked in every mode, except the workspace carve-out
//   - the workspace folder is reachable in every mode
//   - block_all: only the workspace folder
//   - permit_all: everything else
//   - permit_list: built-in allows + the user's allowed folders
func IsPathAllowed(filePath string, policy Policy) bool {
	if insideAny(filePath, builtinAlwaysDenied) {
		return false
	}
	if insideAny(filePath, policy.Protected) {
		return false
	}
	inWorkspace := strings.TrimSpace(policy.WorkspaceRoot) != "" && isInsideFolder(filePath, policy.WorkspaceRoot)
	if strings.TrimSpace(policy.DataDir) != "" && isInsideFolder(filePath, policy.DataDir) && !inWorkspace {
		return false
	}
	if inWorkspace {
		return true
	}
	switch policy.Config.Mode {
	case domain.SandboxPermitAll:
		return true
	case domain.SandboxBlockAll:
		return false
	case domain.SandboxPermitList:
		if insideAny(filePath, builtinAllowedPermitList) {
			return true
		}
		return insideAny(filePath, policy.Config.AllowedFolders)
	}
	// Unknown mode → deny.
	return false
}

// Roots lists the top-level folders currently visible to the agent under the
// policy (port of AW2's getSandboxRoots) — the "Folders visible to the agent"
// summary in the Permissions UI.
func Roots(policy Policy) []string {
	candidates := []string{policy.WorkspaceRoot}
	switch policy.Config.Mode {
	case domain.SandboxPermitList:
		candidates = append(candidates, builtinAllowedPermitList...)
		candidates = append(candidates, policy.Config.AllowedFolders...)
	case domain.SandboxPermitAll:
		candidates = append(candidates, homeDir(), "/")
	}
	seen := map[string]bool{}
	roots := []string{}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		expanded := ExpandPath(candidate)
		if seen[expanded] || !IsPathAllowed(expanded, policy) {
			continue
		}
		seen[expanded] = true
		roots = append(roots, expanded)
	}
	sort.Strings(roots)
	return roots
}
