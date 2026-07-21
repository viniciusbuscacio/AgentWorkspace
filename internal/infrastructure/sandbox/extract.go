package sandbox

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"aw/internal/domain"
)

// Path extraction and the shell pre-flight check, ported from AW2's
// path-validator extractPaths and sandbox-shell sandboxExec. Heuristic by
// design — see the package comment for the honest threat model.

var (
	// Redirect targets: > /path, >> /path, 2> /path, 2>> /path.
	redirectPattern = regexp.MustCompile(`\d?>{1,2}\s*([^\s;|&<>]+)`)
	// curl -o / wget -O download targets.
	curlPattern      = regexp.MustCompile(`(?i)curl\s+.*?-o\s+([^\s;|&<>]+)`)
	wgetPattern      = regexp.MustCompile(`(?i)wget\s+.*?-O\s+([^\s;|&<>]+)`)
	drivePathPattern = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	// Shell operators that disqualify a command from the safe no-path list.
	unsafeOperators = regexp.MustCompile("[\n;|&<>`$(){}\\[\\]\\\\]")
)

// safeNoPathCommands may run in permit_list mode without any verifiable
// path, as long as the command has no shell operators.
var safeNoPathCommands = map[string]bool{
	"date":   true,
	"echo":   true,
	"false":  true,
	"id":     true,
	"printf": true,
	"pwd":    true,
	"true":   true,
	"uname":  true,
	"whoami": true,
}

// ExtractPaths extracts the file paths referenced by a shell command:
// redirect targets, curl/wget download targets, and any token that looks
// like a path (/, ~/, ./, ../), quote-aware. Best-effort heuristic.
func ExtractPaths(command string) []string {
	seen := map[string]bool{}
	paths := []string{}
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	for _, match := range redirectPattern.FindAllStringSubmatch(command, -1) {
		addIfPath(match[1], add)
	}
	for _, match := range curlPattern.FindAllStringSubmatch(command, -1) {
		addIfPath(match[1], add)
	}
	for _, match := range wgetPattern.FindAllStringSubmatch(command, -1) {
		addIfPath(match[1], add)
	}
	for _, token := range tokenize(command) {
		cleaned := strings.Trim(token, `'"`)
		if looksLikePath(cleaned) {
			add(cleaned)
		}
	}
	return paths
}

func addIfPath(path string, add func(string)) {
	cleaned := strings.Trim(path, `'"`)
	if looksLikePath(cleaned) {
		add(cleaned)
	}
}

// tokenize splits a command on whitespace and the |, ;, & operators while
// respecting single quotes, double quotes and backslash escapes.
func tokenize(command string) []string {
	tokens := []string{}
	var current strings.Builder
	inSingle, inDouble, escape := false, false, false
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, ch := range command {
		if escape {
			current.WriteRune(ch)
			escape = false
			continue
		}
		if ch == '\\' {
			escape = true
			current.WriteRune(ch)
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			current.WriteRune(ch)
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			current.WriteRune(ch)
			continue
		}
		if !inSingle && !inDouble {
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
				flush()
				continue
			}
			if ch == '|' || ch == ';' || ch == '&' {
				flush()
				continue
			}
		}
		current.WriteRune(ch)
	}
	flush()
	return tokens
}

// looksLikePath reports whether a token plausibly names a filesystem path.
func looksLikePath(s string) bool {
	if s == "" || s == "/" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return false
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~/") ||
		strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return true
	}
	if drivePathPattern.MatchString(s) || strings.HasPrefix(s, `\\`) {
		return true
	}
	if strings.Contains(s, `\`) && !strings.ContainsAny(s, " \t\r\n") {
		parts := strings.Split(s, `\`)
		return len(parts) >= 2 && parts[0] != "" && parts[1] != ""
	}
	return s == "~"
}

// IsSafeNoPathCommand reports whether a command with no extractable paths may
// still run in permit_list mode: a single allowlisted command with no shell
// operators (which could hide filesystem access behind expansion).
func IsSafeNoPathCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}
	if unsafeOperators.MatchString(trimmed) {
		return false
	}
	name := strings.Fields(trimmed)[0]
	if idx := strings.LastIndexByte(name, '/'); idx >= 0 {
		name = name[idx+1:]
	}
	return safeNoPathCommands[name]
}

// CommandCheck is the result of the shell pre-flight: whether the command may
// run, which paths were detected and why it was blocked. The reason string is
// returned to the model verbatim so it can relay the block honestly.
type CommandCheck struct {
	Allowed      bool     `json:"allowed"`
	Paths        []string `json:"paths"`
	BlockedPaths []string `json:"blockedPaths,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

// CheckCommand runs the AW2 sandboxExec decision flow without executing:
// block_all refuses outright; extracted paths are each validated (built-in
// denies hold even in permit_all); a permit_list command with no verifiable
// paths must be on the safe no-path allowlist. Relative extracted paths are
// evaluated against cwd (the directory the command will run in), not the
// process working directory.
func CheckCommand(command string, cwd string, policy Policy) CommandCheck {
	cwd = strings.TrimSpace(cwd)
	if cwd != "" && !IsPathAllowed(cwd, policy) {
		return CommandCheck{
			BlockedPaths: []string{cwd},
			Reason:       fmt.Sprintf("Access denied to cwd: %s", cwd),
		}
	}
	if policy.Config.Mode == domain.SandboxBlockAll {
		return CommandCheck{
			Reason: "SANDBOX BLOCK_ALL: Shell commands are completely disabled. Do NOT retry. Tell the user to change Permissions in Settings if they need shell commands executed.",
		}
	}
	paths := ExtractPaths(command)
	if len(paths) == 0 {
		if policy.Config.Mode == domain.SandboxPermitList && !IsSafeNoPathCommand(command) {
			return CommandCheck{
				Reason: "SANDBOX PERMIT_LIST: command has no verifiable file paths and is not on the safe no-path allowlist. Do NOT retry. Ask the user to allow a specific folder or run a simpler safe command.",
			}
		}
		return CommandCheck{Allowed: true, Paths: paths}
	}
	blocked := []string{}
	for _, path := range paths {
		if !IsPathAllowed(resolveAgainstCwd(path, cwd), policy) {
			blocked = append(blocked, path)
		}
	}
	if len(blocked) > 0 {
		return CommandCheck{
			Paths:        paths,
			BlockedPaths: blocked,
			Reason:       fmt.Sprintf("Access denied to: %s", strings.Join(blocked, ", ")),
		}
	}
	return CommandCheck{Allowed: true, Paths: paths}
}

// resolveAgainstCwd anchors a relative extracted path to the directory the
// command will execute in. Absolute and ~ paths pass through unchanged.
func resolveAgainstCwd(path string, cwd string) string {
	if cwd == "" {
		return path
	}
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || path == "." || path == ".." {
		return filepath.Join(ExpandPath(cwd), path)
	}
	return path
}
